/*
Copyright © 2024 Marcus Vinicius <mvleandro@gmail.com>

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in
all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
THE SOFTWARE.
*/
package cmd

import (
	"context"
	"fmt"
	"keess/pkg/services"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc" // required for oidc
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// runCmd represents the run command
var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Initiates the synchronization of secrets and configmaps.",
	// Long: `The 'run' command in Keess is the primary trigger for initiating the synchronization process of secrets and configmaps across Kubernetes namespaces and clusters. When executed, it activates Keess’s core functionality, seamlessly transferring specified configurations and sensitive data according to predefined rules and parameters. This command ensures that all targeted Kubernetes environments are updated with the latest configurations and secrets, maintaining consistency and enhancing security across your distributed infrastructure.

	// Features:
	// - Automated Synchronization: Executes the automated process of syncing secrets and configmaps.
	// - Cross-Namespace and Cluster Operation: Works across different namespaces and multiple clusters.
	// - Secure Transfer: Adheres to strict security protocols to ensure safe data transfer.
	// - Custom Synchronization: Respects user-defined rules and conditions for targeted synchronization.
	// - Real-Time Execution: Performs synchronization in real time, ensuring timely updates.
	// - Logging: Generates logs for the synchronization process, aiding in monitoring and auditing.

	// Usage of the 'run' command is essential for keeping your Kubernetes environments synchronized and secure, forming the backbone of Keess's operational capabilities.`,
	Run: func(cmd *cobra.Command, args []string) {
		atom := zap.NewAtomicLevel()
		var level zapcore.Level
		err := level.UnmarshalText([]byte(logLevel))
		if err != nil {
			fmt.Printf("Error setting log level: %s\n", err.Error())
			return
		}
		atom.SetLevel(level)

		cfg, _ := configureLogger()
		logger := zap.New(zapcore.NewCore(
			zapcore.NewJSONEncoder(*cfg),
			zapcore.Lock(os.Stdout),
			atom,
		))
		defer logger.Sync()

		config, err := rest.InClusterConfig()
		if err != nil {
			kubeconfig := filepath.Join(os.Getenv("HOME"), ".kube", "config")
			config, err = buildConfigWithContextFromFlags(localCluster, kubeconfig)
			if err != nil {
				logger.Sugar().Error("Error building localCluster kubeconfig: ", err)
				return
			}
		}

		// create the clientset
		localKubeClient, err := kubernetes.NewForConfig(config)
		if err != nil {
			logger.Sugar().Error("Error creating inCluster clientset: ", err)
			return
		}

		// Create a context
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		namespacePollingInterval, _ := cmd.Flags().GetInt32("namespacePollingInterval")
		pollingInterval, _ := cmd.Flags().GetInt32("pollingInterval")
		housekeepingInterval, _ := cmd.Flags().GetInt32("housekeepingInterval")

		logger.Sugar().Infof("Starting Keess. Running on local cluster: %s", localCluster)
		logger.Sugar().Debugf("Namespace polling interval: %d seconds", namespacePollingInterval)
		logger.Sugar().Debugf("Polling interval: %d seconds", pollingInterval)
		logger.Sugar().Debugf("Housekeeping interval: %d seconds", housekeepingInterval)
		logger.Sugar().Debugf("Log level: %s", logLevel)
		logger.Sugar().Debugf("Kubeconfig path: %s", kubeConfigPath)

		// Create a map of remote clients
		remoteKubeClients := make(map[string]services.IKubeClient)

		if len(remoteClusters) > 0 {

			// Add the remote clientset to the map
			for _, cluster := range remoteClusters {
				remoteClusterConfig, err := buildConfigWithContextFromFlags(cluster, kubeConfigPath)
				if err != nil {
					logger.Sugar().Errorf("Error building remote kubeconfig for cluster '%s': %s", cluster, err)
				}

				remoteClusterClient, err := kubernetes.NewForConfig(remoteClusterConfig)
				if err != nil {
					logger.Sugar().Errorf("Error creating remote clientset for cluster '%s': %s", cluster, err)
				}

				remoteKubeClients[cluster] = remoteClusterClient
			}

			logger.Sugar().Infof("Remote clusters: %v", remoteClusters)
		} else {
			logger.Sugar().Info("No remote clusters to synchronize")
		}

		// Create a NamespacePoller
		namespacePoller := services.NewNamespacePoller(localKubeClient, logger.Sugar())
		namespacePoller.PollNamespaces(ctx, metav1.ListOptions{}, time.Duration(namespacePollingInterval)*time.Second, localCluster)

		// Create a SecretPoller
		secretPoller := services.NewSecretPoller(localCluster, localKubeClient, logger.Sugar())

		// Create a SecretSynchronizer
		secretSynchronizer := services.NewSecretSynchronizer(
			localKubeClient,
			remoteKubeClients,
			secretPoller,
			namespacePoller,
			logger.Sugar(),
		)

		// Start the secret synchronizer
		secretSynchronizer.Start(ctx, time.Duration(pollingInterval)*time.Second, time.Duration(housekeepingInterval)*time.Second)

		// Create a ConfigMapPoller
		configMapPoller := services.NewConfigMapPoller(localCluster, localKubeClient, logger.Sugar())

		// Create a ConfigMapSynchronizer
		configMapSynchronizer := services.NewConfigMapSynchronizer(
			localKubeClient,
			remoteKubeClients,
			configMapPoller,
			namespacePoller,
			logger.Sugar(),
		)

		// Start the configMap synchronizer
		configMapSynchronizer.Start(ctx, time.Duration(pollingInterval)*time.Second, time.Duration(housekeepingInterval)*time.Second)

		// Create an HTTP server and add the health check handler as a handler
		http.HandleFunc("/health", healthHandler)
		http.ListenAndServe(":8080", nil)

		select {}
	},
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	// Check the health of the server and return a status code accordingly
	if serverIsHealthy() {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "Server is healthy")
	} else {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, "Server is not healthy")
	}
}

func serverIsHealthy() bool {
	// Check the health of the server and return true or false accordingly
	// For example, check if the server can connect to the database
	return true
}

var logLevel string
var localCluster string
var remoteClusters []string
var kubeConfigPath string

func init() {
	rootCmd.AddCommand(runCmd)

	// Here you will define your flags and configuration settings.
	runCmd.Flags().StringVarP(&logLevel, "logLevel", "l", "info", "Log level")
	runCmd.Flags().StringVarP(&localCluster, "localCluster", "c", "", "Local cluster name")
	runCmd.Flags().StringVarP(&kubeConfigPath, "kubeConfigPath", "p", "", "Path to the kubeconfig file")
	runCmd.Flags().Int32P("namespacePollingInterval", "n", int32(60), "Interval in seconds to poll the Kubernetes API for namespaces.")
	runCmd.Flags().StringArrayVarP(&remoteClusters, "remoteCluster", "r", []string{}, "Remote cluster to synchronize")
	runCmd.Flags().Int32P("pollingInterval", "s", int32(60), "Interval in seconds to poll the Kubernetes API for secrets and configmaps.")
	runCmd.Flags().Int32P("housekeepingInterval", "k", int32(300), "Interval in seconds to delete orphans objects.")

}

// Configure the logger
func configureLogger() (*zapcore.EncoderConfig, error) {
	cfg := zap.NewProductionEncoderConfig()
	cfg.TimeKey = "timestamp"
	cfg.EncodeTime = zapcore.TimeEncoderOfLayout(time.RFC3339)
	cfg.CallerKey = "caller"

	return &cfg, nil
}

// buildConfigWithContextFromFlags builds a Kubernetes client configuration from the provided context and kubeconfig path.
func buildConfigWithContextFromFlags(context string, kubeconfigPath string) (*rest.Config, error) {
	return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfigPath},
		&clientcmd.ConfigOverrides{
			CurrentContext: context,
		}).ClientConfig()
}
