package service

import (
	v1 "k8s.io/api/core/v1"
	"keess/pkg/keess"
)

// A struct that represents a service in a Kubernetes cluster.
type PacService struct {
	// The name of the cluster where the service is located.
	Cluster string

	// The service.
	Service v1.Service
}

// Prepare the service for persistence as a Cilium Global Service reference.
func (s *PacService) Prepare(namespace string) v1.Service {
	newService := s.Service.DeepCopy()
	newService.Namespace = namespace

	newService.UID = ""
	newService.ResourceVersion = ""
	newService.Labels = map[string]string{}
	newService.Labels[keess.ManagedLabelSelector] = "true"
	newService.Annotations = map[string]string{}
	newService.Annotations[keess.SourceClusterAnnotation] = s.Cluster
	newService.Annotations[keess.SourceNamespaceAnnotation] = s.Service.Namespace
	newService.Annotations[keess.SourceResourceVersionAnnotation] = s.Service.ResourceVersion

	// Add Cilium Global Service annotations
	newService.Annotations[keess.CiliumGlobalServiceAnnotation] = "true"
	newService.Annotations[keess.CiliumSharedServiceAnnotation] = "false"

	// Clear the selector for remote service references (no local endpoints)
	newService.Spec.Selector = map[string]string{}

	// Clear IP information
	newService.Spec.ClusterIP = ""
	newService.Spec.ClusterIPs = []string{}
	newService.Spec.Type = v1.ServiceTypeClusterIP
	// TODO: are more preparations needed if source type is LoadBalancer or NodePort?

	return *newService
}

// Check if the remote service has changed.
func (s *PacService) HasChanged(remote v1.Service) bool {
	if s.Service.ResourceVersion != remote.Annotations[keess.SourceResourceVersionAnnotation] {
		return true
	}

	if remote.Labels[keess.ManagedLabelSelector] != "true" {
		return true
	}

	if remote.Annotations[keess.SourceClusterAnnotation] != s.Cluster {
		return true
	}

	if remote.Annotations[keess.SourceNamespaceAnnotation] != s.Service.Namespace {
		return true
	}

	// Check if Cilium annotations are correct
	if remote.Annotations[keess.CiliumGlobalServiceAnnotation] != "true" {
		return true
	}

	if remote.Annotations[keess.CiliumSharedServiceAnnotation] != "false" {
		return true
	}

	return false
}
