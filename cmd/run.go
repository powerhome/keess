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
	"keess/pkg/keess"
	"keess/pkg/keess/metrics"
	"keess/pkg/keess/service"
	"net/http"
	"os"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc" // required for oidc
	"k8s.io/client-go/rest"
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

		namespacePollingInterval, _ := cmd.Flags().GetInt32("namespacePollingInterval")
		pollingInterval, _ := cmd.Flags().GetInt32("pollingInterval")
		housekeepingInterval, _ := cmd.Flags().GetInt32("housekeepingInterval")
		configReloaderMaxRetries, _ := cmd.Flags().GetInt("configReloaderMaxRetries")
		configReloaderDebounceTimer, _ := cmd.Flags().GetInt("configReloaderDebounceTimer")
		enableServiceSync, _ := cmd.Flags().GetBool("enableServiceSync")

		logger.Sugar().Infof("Starting Keess v%s. Running on local cluster: %s", Version, localCluster)
		logger.Sugar().Debugf("Namespace polling interval: %d seconds", namespacePollingInterval)
		logger.Sugar().Debugf("Polling interval: %d seconds", pollingInterval)
		logger.Sugar().Debugf("Housekeeping interval: %d seconds", housekeepingInterval)
		logger.Sugar().Debugf("Log level: %s", logLevel)
		logger.Sugar().Debugf("Kubeconfig path: %s", kubeConfigPath)
		logger.Sugar().Debugf("Enable service sync: %t", enableServiceSync)

		// Register Prometheus metrics
		metrics.RegisterMetrics()

		config, err := rest.InClusterConfig()
		if err != nil {
			config, err = keess.BuildConfigWithContextFromFlags(localCluster, kubeConfigPath)
			if err != nil {
				logger.Sugar().Error("Error building localCluster kubeconfig: ", err)
				return
			}
		}

		// create the clientset
		localKubeClient, err := keess.NewKubeClientAdapter(config)
		if err != nil {
			logger.Sugar().Error("Error creating local kube client: ", err)
			return
		}

		// Create a context
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Create a map of remote clients
		remoteKubeClients := make(map[string]keess.IKubeClient)

		kubeConfigLoader := keess.NewKubeconfigLoader(kubeConfigPath, logger.Sugar(), remoteKubeClients, configReloaderMaxRetries, configReloaderDebounceTimer)
		kubeConfigLoader.StartWatching(ctx)

		// Create a NamespacePoller
		namespacePoller := keess.NewNamespacePoller(localKubeClient, logger.Sugar())
		namespacePoller.PollNamespaces(ctx, metav1.ListOptions{}, time.Duration(namespacePollingInterval)*time.Second, localCluster)

		// Create a SecretPoller
		secretPoller := keess.NewSecretPoller(localCluster, localKubeClient, logger.Sugar())

		// Create a SecretSynchronizer
		secretSynchronizer := keess.NewSecretSynchronizer(
			localKubeClient,
			remoteKubeClients,
			secretPoller,
			namespacePoller,
			logger.Sugar(),
		)

		// Start the secret synchronizer
		secretSynchronizer.Start(ctx, time.Duration(pollingInterval)*time.Second, time.Duration(housekeepingInterval)*time.Second)

		// Create a ConfigMapPoller
		configMapPoller := keess.NewConfigMapPoller(localCluster, localKubeClient, logger.Sugar())

		// Create a ConfigMapSynchronizer
		configMapSynchronizer := keess.NewConfigMapSynchronizer(
			localKubeClient,
			remoteKubeClients,
			configMapPoller,
			namespacePoller,
			logger.Sugar(),
		)

		// Start the configMap synchronizer
		configMapSynchronizer.Start(ctx, time.Duration(pollingInterval)*time.Second, time.Duration(housekeepingInterval)*time.Second)

		if enableServiceSync {
			// Create a ServicePoller
			servicePoller := service.NewServicePoller(localCluster, localKubeClient, logger.Sugar())

			// Create a ServiceSynchronizer
			serviceSynchronizer := service.NewServiceSynchronizer(
				localKubeClient,
				remoteKubeClients,
				servicePoller,
				namespacePoller,
				logger.Sugar(),
			)

			// Start the service synchronizer
			serviceSynchronizer.Start(ctx, time.Duration(pollingInterval)*time.Second, time.Duration(housekeepingInterval)*time.Second)
		} else {
			logger.Sugar().Info("Service synchronization is disabled. Set --enableServiceSync=true to enable it (depends on Cilium Clustermesh features).")
		}

		// Create an HTTP server and add the health check handler as a handler
		http.HandleFunc("/health", healthHandler)
		http.Handle("/metrics", promhttp.Handler())

		logger.Sugar().Info("Starting HTTP server on :8080 ...")
		if err := http.ListenAndServe(":8080", nil); err != nil {
			logger.Sugar().Fatalf("Failed to start HTTP server: %v", err)
		}
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
var kubeConfigPath string

func init() {
	rootCmd.AddCommand(runCmd)

	// Here you will define your flags and configuration settings.
	runCmd.Flags().StringVarP(&logLevel, "logLevel", "l", "info", "Log level")
	runCmd.Flags().StringVarP(&localCluster, "localCluster", "c", "", "Local cluster name")
	runCmd.Flags().StringVarP(&kubeConfigPath, "kubeConfigPath", "p", "", "Path to the kubeconfig file")
	runCmd.Flags().Int32P("namespacePollingInterval", "n", int32(60), "Interval in seconds to poll the Kubernetes API for namespaces.")
	runCmd.Flags().Int32P("pollingInterval", "s", int32(60), "Interval in seconds to poll the Kubernetes API for secrets and configmaps.")
	runCmd.Flags().Int32P("housekeepingInterval", "k", int32(300), "Interval in seconds to delete orphans objects.")
	runCmd.Flags().IntP("configReloaderMaxRetries", "r", 60, "Max retries for kubeconfig reloader. Each retry will wait 2 second before trying again.")
	runCmd.Flags().IntP("configReloaderDebounceTimer", "d", 500, "Debounce timer for kubeconfig reloader in milliseconds. Each retry will wait this time before trying again.")
	runCmd.Flags().Bool("enableServiceSync", false, "Enable service synchronization. Depends on Cilium Clustermesh features. (default: false)")

	runCmd.MarkFlagRequired("localCluster")
	runCmd.MarkFlagRequired("kubeConfigPath")

}

// Configure the logger
func configureLogger() (*zapcore.EncoderConfig, error) {
	cfg := zap.NewProductionEncoderConfig()
	cfg.TimeKey = "timestamp"
	cfg.EncodeTime = zapcore.TimeEncoderOfLayout(time.RFC3339)
	cfg.CallerKey = "caller"

	return &cfg, nil
}
