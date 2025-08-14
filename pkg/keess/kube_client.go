package keess

import (
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	v1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
)

type IKubeClient interface {
	CoreV1() v1.CoreV1Interface
	Discovery() discovery.DiscoveryInterface
	Dynamic() dynamic.Interface
	ServerVersion() (*version.Info, error)
}

type kubeClientAdapter struct {
	clientset     *kubernetes.Clientset
	dynamicClient dynamic.Interface
}

func (k *kubeClientAdapter) CoreV1() v1.CoreV1Interface {
	return k.clientset.CoreV1()
}

func (k *kubeClientAdapter) Discovery() discovery.DiscoveryInterface {
	return k.clientset.Discovery()
}

func (k *kubeClientAdapter) Dynamic() dynamic.Interface {
	return k.dynamicClient
}

func (k *kubeClientAdapter) ServerVersion() (*version.Info, error) {
	return k.clientset.ServerVersion()
}

func NewKubeClientAdapter(config *rest.Config) (IKubeClient, error) {
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return &kubeClientAdapter{
		clientset:     clientset,
		dynamicClient: dynamicClient,
	}, nil
}
