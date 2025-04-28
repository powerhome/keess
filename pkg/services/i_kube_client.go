package services

import (
	"k8s.io/apimachinery/pkg/version"
	v1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

type IKubeClient interface {
	CoreV1() v1.CoreV1Interface
	ServerVersion() (*version.Info, error)
}
