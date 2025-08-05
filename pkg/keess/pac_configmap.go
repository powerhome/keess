package keess

import (
	v1 "k8s.io/api/core/v1"
)

// A struct that represents a configMap in a Kubernetes cluster.
type PacConfigMap struct {
	// The name of the cluster where the configMap is located.
	Cluster string

	// The configMap.
	ConfigMap v1.ConfigMap
}

// Prepare the configMap for persistence.
func (s *PacConfigMap) Prepare(namespace string) v1.ConfigMap {
	newConfigMap := s.ConfigMap.DeepCopy()
	newConfigMap.Namespace = namespace

	newConfigMap.UID = ""
	newConfigMap.Labels = map[string]string{}
	newConfigMap.Labels[ManagedLabelSelector] = "true"
	newConfigMap.Annotations = map[string]string{}
	newConfigMap.Annotations[SourceClusterAnnotation] = s.Cluster
	newConfigMap.Annotations[SourceNamespaceAnnotation] = s.ConfigMap.Namespace
	newConfigMap.Annotations[SourceResourceVersionAnnotation] = s.ConfigMap.ResourceVersion
	newConfigMap.ResourceVersion = ""

	return *newConfigMap
}

// Check if the remote configMap has changed.
func (s *PacConfigMap) HasChanged(remote v1.ConfigMap) bool {
	if s.ConfigMap.ResourceVersion != remote.Annotations[SourceResourceVersionAnnotation] {
		return true
	}

	if remote.Labels[ManagedLabelSelector] != "true" {
		return true
	}

	if remote.Annotations[SourceClusterAnnotation] != s.Cluster {
		return true
	}

	if remote.Annotations[SourceNamespaceAnnotation] != s.ConfigMap.Namespace {
		return true
	}

	return false
}
