package services

import v1 "k8s.io/api/core/v1"

// PacSecret é uma estrutura representando um Secret do Kubernetes.
type PacSecret struct {
	Secret *v1.Secret
}
