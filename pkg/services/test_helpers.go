package services

import (
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	v1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

type kubeClientAdapter struct {
	clientset *kubernetes.Clientset
}

func (k *kubeClientAdapter) CoreV1() v1.CoreV1Interface {
	return k.clientset.CoreV1()
}

func (k *kubeClientAdapter) ServerVersion() (*version.Info, error) {
	return k.clientset.Discovery().ServerVersion()
}

func newKubeClientAdapter(clientset *kubernetes.Clientset) IKubeClient {
	return &kubeClientAdapter{clientset: clientset}
}

type mockKubeClient struct {
	*fake.Clientset
}

func (m *mockKubeClient) ServerVersion() (*version.Info, error) {
	return &version.Info{
		Major:      "1",
		Minor:      "32.2",
		GitVersion: "v1.32.2",
	}, nil
}
