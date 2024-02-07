package services

import (
	corev1 "k8s.io/api/core/v1"
)

// Extends corev1.Namespace adding PAC context on that.
type PacNamespace struct {
	Namespace *corev1.Namespace
	Cluster   string
}
