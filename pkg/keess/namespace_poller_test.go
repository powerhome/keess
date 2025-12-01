package keess

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestNamespacePoller_PollNamespaces(t *testing.T) {
	mockKubeClient := &MockKubeClient{Clientset: fake.NewSimpleClientset()}
	logger, _ := zap.NewProduction()
	sugaredLogger := logger.Sugar()

	namespacePoller := &NamespacePoller{
		kubeClient: mockKubeClient,
		logger:     sugaredLogger,
		Namespaces: make(map[string]*PacNamespace),
		startup:    true,
	}

	opts := metav1.ListOptions{}
	pollInterval := time.Second * 5
	cluster := "test-cluster"

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create test namespaces
	testNamespaces := []*v1.Namespace{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "namespace1",
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "namespace2",
			},
		},
	}

	// Add test namespaces to the fake clientset
	for _, ns := range testNamespaces {
		_, err := mockKubeClient.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})
		assert.NoError(t, err, "Failed to create test namespace")
	}

	err := namespacePoller.PollNamespaces(ctx, opts, pollInterval, cluster)
	assert.NoError(t, err, "PollNamespaces should not return an error")

	// Wait for the first poll to complete
	time.Sleep(2 * time.Second)

	// Verify that all test namespaces are in the map
	namespacePoller.mutex.Lock()
	defer namespacePoller.mutex.Unlock()

	assert.Equal(t, len(testNamespaces), len(namespacePoller.Namespaces), "Expected all namespaces to be polled")

	for _, ns := range testNamespaces {
		pacNs, exists := namespacePoller.Namespaces[ns.Name]
		assert.True(t, exists, "Test namespace not found in map: %s", ns.Name)
		assert.Equal(t, cluster, pacNs.Cluster, "Namespace cluster should match")
		assert.Equal(t, ns.Name, pacNs.Namespace.Name, "Namespace name should match")
	}
}

func TestNamespacePoller_PollNamespaces_ErrorRecovery(t *testing.T) {
	// Create a client that will fail the first 2 List operations, then succeed
	fakeClient := fake.NewSimpleClientset()
	mockKubeClient := NewErrorInjectingMockKubeClient(fakeClient, 2)

	logger := zap.NewNop().Sugar()

	namespacePoller := &NamespacePoller{
		kubeClient: mockKubeClient,
		logger:     logger,
		Namespaces: make(map[string]*PacNamespace),
		startup:    true,
	}

	opts := metav1.ListOptions{}
	pollInterval := 500 * time.Millisecond
	cluster := "test-cluster"

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Create test namespaces
	testNamespace := &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-namespace",
		},
	}

	_, err := fakeClient.CoreV1().Namespaces().Create(ctx, testNamespace, metav1.CreateOptions{})
	assert.NoError(t, err, "Failed to create test namespace")

	err = namespacePoller.PollNamespaces(ctx, opts, pollInterval, cluster)
	assert.NoError(t, err, "PollNamespaces should not return an error")

	// Wait for multiple poll cycles (enough to go through errors and success)
	// First poll: startup (immediate) - will fail (error 1)
	// Second poll: after 500ms - will fail (error 2)
	// Third poll: after 500ms - will succeed
	time.Sleep(2 * time.Second)

	// Verify that the goroutine survived the errors and successfully polled
	namespacePoller.mutex.Lock()
	defer namespacePoller.mutex.Unlock()

	assert.NotEmpty(t, namespacePoller.Namespaces, "Namespaces should be populated after error recovery")
	pacNs, exists := namespacePoller.Namespaces[testNamespace.Name]
	assert.True(t, exists, "Test namespace should exist after error recovery")
	if exists {
		assert.Equal(t, cluster, pacNs.Cluster, "Namespace cluster should match")
		assert.Equal(t, testNamespace.Name, pacNs.Namespace.Name, "Namespace name should match")
	}
}
