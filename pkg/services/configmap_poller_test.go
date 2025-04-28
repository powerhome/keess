package services

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
	mockKubeClient := &mockKubeClient{Clientset: fake.NewSimpleClientset()}
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

// GroupVersionKind is a mock method for testing purposes
func (ps *PacConfigMap) GroupVersionKind() schema.GroupVersionKind {
	return schema.GroupVersionKind{}
}
