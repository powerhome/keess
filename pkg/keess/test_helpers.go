package keess

import (
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes/fake"
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
