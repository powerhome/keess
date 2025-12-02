package keess

import (
	"fmt"
	"sync/atomic"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/testing"
)

type MockKubeClient struct {
	*fake.Clientset
	dynamicClient dynamic.Interface
}

func (m *MockKubeClient) Discovery() discovery.DiscoveryInterface {
	return m.Clientset.Discovery()
}

func (m *MockKubeClient) Dynamic() dynamic.Interface {
	return m.dynamicClient
}

func (m *MockKubeClient) ServerVersion() (*version.Info, error) {
	return &version.Info{
		Major:      "1",
		Minor:      "32.2",
		GitVersion: "v1.32.2",
	}, nil
}

// ErrorInjectingMockKubeClient is a mock client that can inject errors for testing
type ErrorInjectingMockKubeClient struct {
	*MockKubeClient
	errorCount   int32 // number of times to return error before succeeding
	currentCount int32 // current error count
}

// NewErrorInjectingMockKubeClient creates a mock client that will fail the first 'errorCount' List operations
func NewErrorInjectingMockKubeClient(clientset *fake.Clientset, errorCount int) *ErrorInjectingMockKubeClient {
	mock := &ErrorInjectingMockKubeClient{
		MockKubeClient: &MockKubeClient{Clientset: clientset},
		errorCount:     int32(errorCount),
		currentCount:   0,
	}

	// Add reactor to inject errors on List operations
	clientset.PrependReactor("list", "*", func(action testing.Action) (bool, runtime.Object, error) {
		current := atomic.AddInt32(&mock.currentCount, 1)
		if current <= mock.errorCount {
			return true, nil, fmt.Errorf("simulated transient error (attempt %d/%d)", current, mock.errorCount)
		}
		// Allow normal processing after error count is exhausted
		return false, nil, nil
	})

	return mock
}
