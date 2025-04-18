package services

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/fsnotify/fsnotify"
	"go.uber.org/zap"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// KubeconfigLoader is a struct that can (re-)load the Kubeconfig from the local filesystem.
type KubeconfigLoader struct {
	path              string
	logger            *zap.SugaredLogger
	watcher           *fsnotify.Watcher
	lastConfigHash    string
	httpClient        *http.Client
	remoteKubeClients map[string]IKubeClient
}

// NewKubeconfigLoader creates a new KubeconfigLoader.
func NewKubeconfigLoader(kubeConfigPath string, logger *zap.SugaredLogger, remoteKubeClients map[string]IKubeClient) *KubeconfigLoader {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		logger.Error("Error creating fsnotify watcher: ", err)
		return nil
	}
	return &KubeconfigLoader{
		path:              kubeConfigPath,
		logger:            logger,
		watcher:           watcher,
		remoteKubeClients: remoteKubeClients,
		lastConfigHash:    "",
	}
}

// Cleanup closes the watcher and logs an error if it fails.
func (k *KubeconfigLoader) Cleanup() {
	defer k.logger.Info("Kubeconfig watcher closed")
	if err := k.watcher.Close(); err != nil {
		k.logger.Errorf("Error closing watcher: %s", err)
	}
}

// hasKubeconfigChanged checks if the kubeconfig file has changed by comparing its hash.
func (k *KubeconfigLoader) hasKubeconfigChanged() (bool, string, error) {
	content, err := os.ReadFile(k.path)
	if err != nil {
		return false, "", fmt.Errorf("error reading kubeconfig file: %w", err)
	}

	hash := fmt.Sprintf("%x", sha256.Sum256(content))

	return hash != k.lastConfigHash, hash, nil
}

// LoadKubeconfig loads the kubeconfig from the filesystem and initializes remote clusters.
func (k *KubeconfigLoader) LoadKubeconfig() {
	changed, currentHash, err := k.hasKubeconfigChanged()
	if err != nil {
		k.logger.Errorf("Failed to check kubeconfig changes: %s", err)
		return
	}
	if !changed {
		k.logger.Debugf("No changes detected in kubeconfig %s, skipping reload", k.path)
		return
	}
	k.logger.Debugf("Detected kubeconfig change, reloading: %s", k.path)
	for client := range k.remoteKubeClients { // if we reassing the map, the synchronizers lose the reference to it
		delete(k.remoteKubeClients, client)
		k.logger.Debugf("Removed remote client for cluster: %s", client)
	}

	// Update stored hash
	k.lastConfigHash = currentHash

	kubeConfig, err := clientcmd.LoadFromFile(k.path)
	if err != nil {
		k.logger.Errorf("Error loading kube config from path %s: %s", k.path, err)
		return
	}
	if kubeConfig == nil {
		k.logger.Errorf("Loaded kubeconfig from %s is empty or invalid", k.path)
		return
	}

	var remoteClustersName []string
	for contextName := range kubeConfig.Contexts {
		remoteClustersName = append(remoteClustersName, contextName)
	}
	k.logger.Debugf("Remote clusters found in kubeconfig: %v", remoteClustersName)

	if len(remoteClustersName) > 0 {
		var initializedClustersName []string
		for _, cluster := range remoteClustersName {
			remoteClusterConfig, err := BuildConfigWithContextFromFlags(cluster, k.path)
			if err != nil {
				k.logger.Errorf("Error building kubeconfig for cluster '%s': %s", cluster, err)
				continue
			}

			remoteClusterConfig.Timeout = 1 * time.Second // Set a timeout for the HTTP client, maybe this should be configurable
			remoteClusterClient, err := kubernetes.NewForConfig(remoteClusterConfig)
			if err != nil {
				k.logger.Errorf("Error creating remote clientset for cluster '%s': %s", cluster, err)
				continue
			}

			output, err := remoteClusterClient.ServerVersion()
			// This is a simple way to check if the server is reachable and the config is valid
			if err != nil {
				k.logger.Errorf("Error getting server version for cluster '%s': %s", cluster, err)
				continue
			}
			k.logger.Infof("Connected to remote cluster '%s' with server version: %s", cluster, output.String())

			k.remoteKubeClients[cluster] = remoteClusterClient
			initializedClustersName = append(initializedClustersName, cluster)
			k.logger.Debugf("Initialized remote cluster client for '%s'", cluster)
		}

		k.logger.Infof("Remote clusters successfully initialized: %v", initializedClustersName)
	} else {
		k.logger.Info("No remote clusters to synchronize")
	}
}

// StartWatching starts watching the kubeconfig file for changes, including deletions and recreations.
func (k *KubeconfigLoader) StartWatching(ctx context.Context) {
	var debounceTimer *time.Timer // timer to debounce events and avoid multiple reloads
	debounceDuration := 500 * time.Millisecond
	go func() {
		_, err := os.Stat(k.path)
		for err != nil && os.IsNotExist(err) {
			k.logger.Warnf("Kubeconfig file does not exist yet: %s", k.path)
			time.Sleep(2 * time.Second) // Polling interval for file existence
			_, err = os.Stat(k.path)
		}
		k.logger.Infof("Kubeconfig file found, starting watcher: %s", k.path)
		if err := k.watcher.Add(k.path); err != nil {
			k.logger.Errorf("Error adding watcher for kubeconfig: %s", err)
			return
		}
		k.logger.Debugf("Added watcher for kubeconfig file: %s", k.path)
		k.LoadKubeconfig()

		for {
			select {
			case event, ok := <-k.watcher.Events:
				if !ok {
					return
				}

				if event.Op&(fsnotify.Write|fsnotify.Create) != 0 {
					// Reset debounce timer if another event comes in quickly
					if debounceTimer != nil {
						debounceTimer.Stop()
					}

					debounceTimer = time.AfterFunc(debounceDuration, func() { // this is necessary because "depending on the editor and OS, modifying a file can trigger multiple events": https://github.com/fsnotify/fsnotify/issues/344
						k.logger.Debug("Detected kubeconfig file operation: ", event.Op)
						k.LoadKubeconfig()
					})
				}

				if event.Op&fsnotify.Remove != 0 { // TBH, I don't know if this is necessary, but it doesn't hurt
					k.logger.Warnf("Kubeconfig file was removed: %s", event.Name)
					k.watcher.Remove(k.path)
					k.logger.Debug("Removed watcher for kubeconfig file due to deletion: %s", k.path)

					// Attempt to re-add the watcher when the file is recreated
					go func() {
						k.logger.Infof("Waiting for kubeconfig file to be recreated: %s", k.path)
						maxRetries := 60
						for i := 0; i < maxRetries; i++ {
							if _, err := clientcmd.LoadFromFile(k.path); err == nil {
								k.logger.Infof("Kubeconfig file recreated, reloading: %s", k.path)
								k.LoadKubeconfig()
								if err := k.watcher.Add(k.path); err != nil {
									k.logger.Errorf("Failed to re-add watcher for kubeconfig: %s", err)
								} else {
									k.logger.Debugf("Re-added watcher for kubeconfig file: %s", k.path)
								}
								return
							}
							time.Sleep(2 * time.Second) // Polling interval for file recreation
						}
						k.logger.Warn("Kubeconfig file was not recreated within the timeout period. Stopping watcher.")
					}()
				}

			case <-ctx.Done():
				k.logger.Warn("Kubeconfig watcher stopped by context cancellation.")
				k.Cleanup()

			case err, ok := <-k.watcher.Errors:
				if !ok {
					return
				}
				k.logger.Errorf("Watcher error: %s", err)
			}
		}
	}()
}

// BuildConfigWithContextFromFlags builds a Kubernetes client configuration from the provided context and kubeconfig path.
func BuildConfigWithContextFromFlags(context string, kubeconfigPath string) (*rest.Config, error) {
	return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfigPath},
		&clientcmd.ConfigOverrides{
			CurrentContext: context,
		}).ClientConfig()
}
