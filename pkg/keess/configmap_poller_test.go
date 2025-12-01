package keess

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/fake"
)

func TestConfigMapPoller_PollConfigMaps(t *testing.T) {
	cluster := "test-cluster"
	mockKubeClient := &MockKubeClient{Clientset: fake.NewSimpleClientset()}
	logger, _ := zap.NewProduction()
	sugaredLogger := logger.Sugar()

	configMapPoller := &ConfigMapPoller{
		cluster:    cluster,
		kubeClient: mockKubeClient,
		logger:     sugaredLogger,
		startup:    true,
	}

	opts := metav1.ListOptions{}
	pollInterval := time.Second * 5

	ctx := context.Background()

	configMapsChan, err := configMapPoller.PollConfigMaps(ctx, opts, pollInterval)
	assert.NoError(t, err, "PollConfigMaps should not return an error")

	// Create test configMaps
	testConfigMaps := []*v1.ConfigMap{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "configMap1",
				Namespace: "default",
			},
			Data: map[string]string{
				"key1": string([]byte("value1")),
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "configMap2",
				Namespace: "default",
			},
			Data: map[string]string{
				"key2": string([]byte("value2")),
			},
		},
	}

	// Add test configMaps to the fake clientset
	for _, configMap := range testConfigMaps {
		_, err := mockKubeClient.CoreV1().ConfigMaps(configMap.Namespace).Create(ctx, configMap, metav1.CreateOptions{})
		assert.NoError(t, err, "Failed to create test configMap")
	}

	receivedConfigMaps := make(map[string]bool)
	time.Sleep(2 * time.Second)

	// Verify that the configMaps are received on the channel
	go func() {
		for configMap := range configMapsChan {
			receivedConfigMaps[configMap.ConfigMap.Name] = true
		}
	}()

	// Wait for the configMaps to be received
	for {
		if len(receivedConfigMaps) == len(testConfigMaps) {
			break
		}
		time.Sleep(1 * time.Second)
	}

	// Verify that all test configMaps are received
	for _, configMap := range testConfigMaps {
		assert.True(t, receivedConfigMaps[configMap.Name], "Test configMap not received")
	}

	ctx.Done()
}

func TestConfigMapPoller_PollConfigMaps_ErrorRecovery(t *testing.T) {
	cluster := "test-cluster"

	// Create a client that will fail the first 2 List operations, then succeed
	fakeClient := fake.NewSimpleClientset()
	mockKubeClient := NewErrorInjectingMockKubeClient(fakeClient, 2)

	logger := zap.NewNop().Sugar()

	configMapPoller := &ConfigMapPoller{
		cluster:    cluster,
		kubeClient: mockKubeClient,
		logger:     logger,
		startup:    true,
	}

	opts := metav1.ListOptions{}
	pollInterval := 500 * time.Millisecond

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Create test configMap
	testConfigMap := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-configmap",
			Namespace: "default",
		},
		Data: map[string]string{
			"key": "value",
		},
	}

	_, err := fakeClient.CoreV1().ConfigMaps(testConfigMap.Namespace).Create(ctx, testConfigMap, metav1.CreateOptions{})
	assert.NoError(t, err, "Failed to create test configMap")

	configMapsChan, err := configMapPoller.PollConfigMaps(ctx, opts, pollInterval)
	assert.NoError(t, err, "PollConfigMaps should not return an error")

	receivedConfigMaps := make(map[string]bool)
	done := make(chan bool)

	// Collect configMaps from the channel
	go func() {
		for configMap := range configMapsChan {
			receivedConfigMaps[configMap.ConfigMap.Name] = true
		}
		done <- true
	}()

	// Wait for multiple poll cycles (enough to go through errors and success)
	// First poll: startup (immediate) - will fail (error 1)
	// Second poll: after 500ms - will fail (error 2)
	// Third poll: after 500ms - will succeed
	time.Sleep(2 * time.Second)

	// Cancel context to stop polling
	cancel()

	// Wait for channel to close
	select {
	case <-done:
		// Channel closed successfully
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for channel to close")
	}

	// Verify that the goroutine survived the errors and successfully polled
	assert.True(t, receivedConfigMaps[testConfigMap.Name], "ConfigMap should be received after error recovery")
}

// GroupVersionKind is a mock method for testing purposes
func (ps *PacConfigMap) GroupVersionKind() schema.GroupVersionKind {
	return schema.GroupVersionKind{}
}
