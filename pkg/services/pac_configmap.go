package services

import v1 "k8s.io/api/core/v1"

// PacConfigMap é uma estrutura representando um ConfigMap do Kubernetes.
type PacConfigMap struct {
	ConfigMap *v1.ConfigMap
}
