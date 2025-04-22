package services

import (
	"testing"

	"github.com/fsnotify/fsnotify"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
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
		description       string
		kubeConfigPath    string
		expectedLogs      []string
		remoteKubeClients map[string]IKubeClient
		overrideKCL       *KubeconfigLoader
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
				"No contexts found in kubeconfig file ./fixtures/kubeconfig_empty.yaml",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			mockLogger, observedLogs := newMockLogger()
			kcl := NewKubeconfigLoader(tc.kubeConfigPath, mockLogger, tc.remoteKubeClients)

			if tc.overrideKCL != nil {
				kcl.lastConfigHash = tc.overrideKCL.lastConfigHash
			}

			kcl.LoadKubeconfig()
			kcl.logger.Sync()
			logs := observedLogs.All()
			if len(logs) != len(tc.expectedLogs) {
				t.Errorf("Expected %d log entries, got %d", len(tc.expectedLogs), len(logs))
			}
			for i, log := range logs {
				if log.Message != tc.expectedLogs[i] {
					t.Errorf("Expected log message '%s', got '%s'", tc.expectedLogs[i], log.Message)
				}
			}
		})
	}
}
