package services

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	v1 "k8s.io/api/core/v1"
)

// A struct that represents a secret in a Kubernetes cluster.
type PacSecret struct {
	// The name of the cluster where the secret is located.
	Cluster string

	// The secret.
	Secret v1.Secret
}

// Prepare the secret for persistence.
func (s *PacSecret) Prepare(namespace string) v1.Secret {
	newSecret := s.Secret.DeepCopy()
	newSecret.Namespace = namespace

	newSecret.UID = ""
	newSecret.Labels = map[string]string{}
	newSecret.Labels[ManagedLabelSelector] = "true"
	newSecret.Annotations = map[string]string{}
	newSecret.Annotations[SourceClusterAnnotation] = s.Cluster
	newSecret.Annotations[SourceNamespaceAnnotation] = s.Secret.Namespace
	newSecret.Annotations[SourceResourceVersionAnnotation] = s.Secret.ResourceVersion
	newSecret.ResourceVersion = ""

	return *newSecret
}

// Check if the remote secret has changed.
func (s *PacSecret) HasChanged(remote v1.Secret) bool {
	if s.Secret.ResourceVersion != remote.Annotations[SourceResourceVersionAnnotation] {
		return true
	}

	if remote.Labels[ManagedLabelSelector] != "true" {
		return true
	}

	if remote.Annotations[SourceClusterAnnotation] != s.Cluster {
		return true
	}

	if remote.Annotations[SourceNamespaceAnnotation] != s.Secret.Namespace {
		return true
	}

	if Digest(s.Secret.Data) != Digest(remote.Data) {
		return true
	}

	return false
}

func Digest(data map[string][]byte) string {
	contents, err := json.Marshal(data)
	if err != nil {
		// TODO: What should we do if we fail to marshall the data
		// property for digestion?
		return ""
	}

	return fmt.Sprintf("%x", sha256.Sum256(contents))
}
