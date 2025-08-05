package services

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
	"k8s.io/client-go/rest"
)

func newMockLogger() (*zap.SugaredLogger, *observer.ObservedLogs) {
	core, observed := observer.New(zap.DebugLevel)
	logger := zap.New(core).Sugar()
	return logger, observed
}

func newMockWatcher() *fsnotify.Watcher {
	mockWatcher, _ := fsnotify.NewWatcher()
	return mockWatcher
}

func TestKubeconfigLoader_NewKubeconfigLoader(t *testing.T) {
	mockLogger, _ := newMockLogger()
	kcl := NewKubeconfigLoader("/kubeconfig/path", mockLogger, nil, 0, 0)
	assert.NotNil(t, kcl, "KubeconfigLoader should not be nil")
	assert.Equal(t, "/kubeconfig/path", kcl.path, "KubeconfigLoader path should match the provided path")
	assert.NotNil(t, kcl.watcher, "KubeconfigLoader watcher should not be nil")
	assert.Equal(t, mockLogger, kcl.logger, "KubeconfigLoader logger should match the provided logger")
	assert.Empty(t, kcl.remoteKubeClients.clients, "KubeconfigLoader remoteKubeClients should be empty")
	assert.Empty(t, kcl.lastConfigHash, "KubeconfigLoader lastConfigHash should be empty")
	assert.Nil(t, kcl.clientFactory, "KubeconfigLoader clientFactory should be nil")
	assert.Equal(t, 0, kcl.maxRetries, "KubeconfigLoader maxRetries should be 0")
	assert.Equal(t, time.Duration(0), kcl.debounceDuration, "KubeconfigLoader debounceDuration should be 0")
}

func TestKubeconfigLoader_Cleanup(t *testing.T) {
	mockWatcher := newMockWatcher()
	mockLogger, observedLogs := newMockLogger()

	kcl := &KubeconfigLoader{
		watcher: mockWatcher,
		logger:  mockLogger,
	}

	kcl.Cleanup()
	kcl.logger.Sync()
	logs := observedLogs.All()
	if len(logs) != 1 {
		t.Errorf("Expected 1 log entry, got %d", len(logs))
	}
	if logs[0].Message != "Kubeconfig watcher closed" {
		t.Errorf("Expected log message 'Kubeconfig watcher closed', got '%s'", logs[0].Message)
	}
}

func TestKubeconfigLoader_LoadKubeconfig(t *testing.T) {
	testCases := []struct {
		description        string
		kubeConfigPath     string
		remoteKubeClients  map[string]IKubeClient
		expectedLogs       []string
		overrideKCL        *KubeconfigLoader
		shouldHaveContexts bool
		expectedContexts   []string
	}{
		{
			description:    "should error with invalid kubeconfig path",
			kubeConfigPath: "/invalid_path",
			expectedLogs: []string{
				"Failed to check kubeconfig changes: error reading kubeconfig file: open /invalid_path: no such file or directory",
			},
		},
		{
			description:    "should exit with kubeconfig with same hash",
			kubeConfigPath: "./fixtures/kubeconfig_empty.yaml",
			expectedLogs: []string{
				"No changes detected in kubeconfig ./fixtures/kubeconfig_empty.yaml, skipping reload",
			},
			overrideKCL: &KubeconfigLoader{
				lastConfigHash: "fd7ac3e961b70cee118473c502416e803b732b3415aebdf2138c598b61955976",
			},
		},
		{
			description:    "should exit with kubeconfig empty",
			kubeConfigPath: "./fixtures/kubeconfig_empty.yaml",
			expectedLogs: []string{
				"Detected kubeconfig change, reloading: ./fixtures/kubeconfig_empty.yaml",
				"Locked remote clients mutex for cleanup",
				"Unlocked remote clients mutex after cleanup",
				"No contexts found in kubeconfig file: ./fixtures/kubeconfig_empty.yaml",
			},
		},
		{
			description:    "should remove not existing remote client",
			kubeConfigPath: "./fixtures/kubeconfig_empty.yaml",
			expectedLogs: []string{
				"Detected kubeconfig change, reloading: ./fixtures/kubeconfig_empty.yaml",
				"Locked remote clients mutex for cleanup",
				"Removed remote client for cluster: test-cluster",
				"Unlocked remote clients mutex after cleanup",
				"No contexts found in kubeconfig file: ./fixtures/kubeconfig_empty.yaml",
			},
			remoteKubeClients: map[string]IKubeClient{
				"test-cluster": nil,
			},
		},
		{
			description:    "should fail with unreachable remote cluster",
			kubeConfigPath: "./fixtures/kubeconfig_with_test-cluster.yaml",
			expectedLogs: []string{
				"Detected kubeconfig change, reloading: ./fixtures/kubeconfig_with_test-cluster.yaml",
				"Locked remote clients mutex for cleanup",
				"Unlocked remote clients mutex after cleanup",
				"Remote clusters found in kubeconfig: [test-cluster]",
				"Locked remote clients mutex for assignment",
				"Error getting server version for cluster 'test-cluster': Get \"https://127.0.0.1:12345/version?timeout=1s\": dial tcp 127.0.0.1:12345: connect: connection refused",
				"Unlocked remote clients mutex after assignment",
			},
			remoteKubeClients: make(map[string]IKubeClient),
		},
		{
			description:    "should generate kube client from kubeconfig",
			kubeConfigPath: "./fixtures/kubeconfig_with_test-cluster.yaml",
			expectedLogs: []string{
				"Detected kubeconfig change, reloading: ./fixtures/kubeconfig_with_test-cluster.yaml",
				"Locked remote clients mutex for cleanup",
				"Unlocked remote clients mutex after cleanup",
				"Remote clusters found in kubeconfig: [test-cluster]",
				"Locked remote clients mutex for assignment",
				"Connected to remote cluster 'test-cluster' with server version: v1.32.2",
				"Initialized remote cluster client for 'test-cluster'",
				"Remote clusters successfully initialized: [test-cluster]",
				"Unlocked remote clients mutex after assignment",
			},
			remoteKubeClients: make(map[string]IKubeClient),
			overrideKCL: &KubeconfigLoader{
				clientFactory: func(_ *rest.Config) (IKubeClient, error) {
					return &MockKubeClient{}, nil
				},
			},
			shouldHaveContexts: true,
			expectedContexts:   []string{"test-cluster"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			mockLogger, observedLogs := newMockLogger()
			kcl := NewKubeconfigLoader(tc.kubeConfigPath, mockLogger, tc.remoteKubeClients, 0, 0)

			if tc.overrideKCL != nil {
				kcl.lastConfigHash = tc.overrideKCL.lastConfigHash
				kcl.clientFactory = tc.overrideKCL.clientFactory
			}

			kcl.LoadKubeconfig()
			if tc.shouldHaveContexts {
				assert.NotEmpty(t, kcl.remoteKubeClients.clients, "Expected remoteKubeClients.clients to be initialized")
				assert.Equal(t, len(tc.expectedContexts), len(kcl.remoteKubeClients.clients), "Expected remoteKubeClients.clients to match expected contexts")
				for _, context := range tc.expectedContexts {
					_, ok := kcl.remoteKubeClients.clients[context]
					assert.True(t, ok, "Expected remoteKubeClient for context '%s' to be found", context)
				}
			} else {
				assert.Empty(t, kcl.remoteKubeClients.clients, "Expected remoteKubeClients.clients to be empty")
			}

			kcl.logger.Sync()
			logs := observedLogs.All()
			assert.Lenf(t, logs, len(tc.expectedLogs), "Expected %d log entries, got %d", len(tc.expectedLogs), len(logs))
			for i, log := range logs {
				if log.Message != tc.expectedLogs[i] {
					t.Errorf("Expected log message '%s', got '%s'", tc.expectedLogs[i], log.Message)
				}
			}
		})
	}
}

func TestKubeconfigLoader_StartWatching(t *testing.T) {
	testCases := []struct {
		description              string
		kubeConfigPath           string
		remoteKubeClients        map[string]IKubeClient
		timeToWait               int
		createKubeconfig         bool
		expectedLogs             []string
		substituteKubeConfigPath bool
		deleteKubeConfigPath     bool
		recreateKubeConfigPath   bool
		configReloaderMaxRetries int
		expected                 struct {
			lastConfigHash string
		}
	}{
		{
			description:    "should wait for file creation and fail as it reached max retries",
			kubeConfigPath: "./fixtures/non-existent-kubeconfig.yaml",
			timeToWait:     3,
			expectedLogs: []string{
				"Kubeconfig file does not exist yet: ./fixtures/non-existent-kubeconfig.yaml",
				"Error checking kubeconfig file existence: stat ./fixtures/non-existent-kubeconfig.yaml: no such file or directory",
			},
			configReloaderMaxRetries: 1,
		},
		{
			description:      "should wait for file creation and then start watching",
			kubeConfigPath:   "./fixtures/non-existent-kubeconfig.yaml",
			timeToWait:       3,
			createKubeconfig: true,
			expectedLogs: []string{ //
				"Kubeconfig file does not exist yet: ./fixtures/non-existent-kubeconfig.yaml",
				"Kubeconfig file does not exist yet: ./fixtures/non-existent-kubeconfig.yaml",
				"Kubeconfig file found, starting watcher: ./fixtures/non-existent-kubeconfig.yaml",
				"Added watcher for kubeconfig file: ./fixtures/non-existent-kubeconfig.yaml",
				"Detected kubeconfig change, reloading: ./fixtures/non-existent-kubeconfig.yaml",
				"Locked remote clients mutex for cleanup",
				"Unlocked remote clients mutex after cleanup",
				"No contexts found in kubeconfig file: ./fixtures/non-existent-kubeconfig.yaml",
			},
			expected: struct {
				lastConfigHash string
			}{
				lastConfigHash: "0078166d266f99e77f5369b52656a2a4ebee2f619aefa560fd0d2afb41a793c7",
			},
			configReloaderMaxRetries: 10,
		},
		{
			description:    "should successfully load kubeconfig, notice it was deleted and then fail as it reached max retries on polling",
			kubeConfigPath: "./fixtures/kubeconfig_empty.yaml",
			timeToWait:     3,
			expectedLogs: []string{
				"Kubeconfig file found, starting watcher: ./fixtures/temp/generated_kubeconfig.yaml",
				"Added watcher for kubeconfig file: ./fixtures/temp/generated_kubeconfig.yaml",
				"Detected kubeconfig change, reloading: ./fixtures/temp/generated_kubeconfig.yaml",
				"Locked remote clients mutex for cleanup",
				"Unlocked remote clients mutex after cleanup",
				"No contexts found in kubeconfig file: ./fixtures/temp/generated_kubeconfig.yaml",
				"Kubeconfig file was removed: ./fixtures/temp/generated_kubeconfig.yaml",
				"Removed watcher for kubeconfig file due to deletion: ./fixtures/temp/generated_kubeconfig.yaml",
				"Waiting for kubeconfig file to be recreated: ./fixtures/temp/generated_kubeconfig.yaml",
				"Watched kubeconfig file ./fixtures/temp/generated_kubeconfig.yaml was not recreated within the timeout period. Stopping watcher.",
				"Kubeconfig watcher closed",
			},
			substituteKubeConfigPath: true,
			deleteKubeConfigPath:     true,
			expected: struct {
				lastConfigHash string
			}{
				lastConfigHash: "fd7ac3e961b70cee118473c502416e803b732b3415aebdf2138c598b61955976",
			},
			configReloaderMaxRetries: 1,
		},
		{
			description:    "should successfully load kubeconfig, notice it was deleted, wait for its recreation and re-add watcher when its recreated",
			kubeConfigPath: "./fixtures/kubeconfig_empty.yaml",
			timeToWait:     3,
			expectedLogs: []string{
				"Kubeconfig file found, starting watcher: ./fixtures/temp/generated_kubeconfig.yaml",
				"Added watcher for kubeconfig file: ./fixtures/temp/generated_kubeconfig.yaml",
				"Detected kubeconfig change, reloading: ./fixtures/temp/generated_kubeconfig.yaml",
				"Locked remote clients mutex for cleanup",
				"Unlocked remote clients mutex after cleanup",
				"No contexts found in kubeconfig file: ./fixtures/temp/generated_kubeconfig.yaml",
				"Kubeconfig file was removed: ./fixtures/temp/generated_kubeconfig.yaml",
				"Removed watcher for kubeconfig file due to deletion: ./fixtures/temp/generated_kubeconfig.yaml",
				"Waiting for kubeconfig file to be recreated: ./fixtures/temp/generated_kubeconfig.yaml",
				"Kubeconfig file recreated, reloading: ./fixtures/temp/generated_kubeconfig.yaml",
				"Re-added watcher for kubeconfig file: ./fixtures/temp/generated_kubeconfig.yaml",
				"No changes detected in kubeconfig ./fixtures/temp/generated_kubeconfig.yaml, skipping reload",
			},
			substituteKubeConfigPath: true,
			deleteKubeConfigPath:     true,
			recreateKubeConfigPath:   true,
			expected: struct {
				lastConfigHash string
			}{
				lastConfigHash: "fd7ac3e961b70cee118473c502416e803b732b3415aebdf2138c598b61955976",
			},
			configReloaderMaxRetries: 10,
		},
	}

	tempDir := "./fixtures/temp"
	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			mockLogger, observedLogs := newMockLogger()
			var dstFile string
			if tc.substituteKubeConfigPath {
				dstFile = tempDir + "/generated_kubeconfig.yaml"
				src, err := os.ReadFile(tc.kubeConfigPath)
				if err != nil {
					t.Fatalf("Failed to read source kubeconfig file: %v", err)
				}
				err = os.WriteFile(dstFile, src, 0644)
				if err != nil {
					t.Fatalf("Failed to write destination kubeconfig file: %v", err)
				}
				t.Cleanup(func() {
					os.Remove(dstFile)
				})
			} else {
				dstFile = tc.kubeConfigPath
			}
			kcl := NewKubeconfigLoader(dstFile, mockLogger, tc.remoteKubeClients, tc.configReloaderMaxRetries, 0)

			ctx, cancel := context.WithCancel(context.Background())
			kcl.StartWatching(ctx)

			if tc.createKubeconfig {
				time.Sleep(time.Duration(2) * time.Second)
				emptyKubeConfig := []byte("apiVersion: v1\nclusters: []\ncontexts: []\n")
				os.WriteFile(tc.kubeConfigPath, emptyKubeConfig, 0644)
				t.Cleanup(func() {
					os.Remove(tc.kubeConfigPath)
				})
			}

			if tc.deleteKubeConfigPath {
				time.Sleep(time.Duration(3) * time.Second)
				os.Remove(dstFile)
			}

			if tc.recreateKubeConfigPath {
				time.Sleep(time.Duration(3) * time.Second)
				src, err := os.ReadFile(tc.kubeConfigPath)
				if err != nil {
					t.Fatalf("Failed to read source kubeconfig file: %v", err)
				}
				err = os.WriteFile(dstFile, src, 0644)
				if err != nil {
					t.Fatalf("Failed to write destination kubeconfig file: %v", err)
				}
			}

			time.Sleep(time.Duration(tc.timeToWait) * time.Second)
			cancel()

			kcl.logger.Sync()
			logs := observedLogs.All()
			assert.Equal(t, tc.expected.lastConfigHash, kcl.lastConfigHash, "Expected lastConfigHash to match expected value")
			assert.Lenf(t, logs, len(tc.expectedLogs), "Expected %d log entries, got %d", len(tc.expectedLogs), len(logs))
			for i, log := range logs {
				if log.Message != tc.expectedLogs[i] {
					t.Errorf("Expected log message '%s', got '%s'", tc.expectedLogs[i], log.Message)
				}
			}
		},
		)
	}
}
