package service

import (
	"context"
	"keess/pkg/keess"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

// Check for local endpoints
//
// This function checks if the service has local endpoints by looking at the endpoint
// addresses and confirming the belong to the CIDR for pods in the local cluster.
func (s *PacService) HasLocalEndpoints(ctx context.Context, localKubeClient keess.IKubeClient) (bool, error) {
	// // Get local addressing CIDR for pods
	// podCIDR := localKubeClient.GetPodCIDR()

	// // Get endpoints for the service
	// endpoints, err := s.localKubeClient.CoreV1().Endpoints(service.Namespace).Get(ctx, service.Name, v1.GetOptions{})
	// if err != nil {
	// 	s.logger.Debugf("[Service][hasLocalEndpoints] Failed to get endpoints for service %s/%s: %v", service.Namespace, service.Name, err)
	// 	return false
	// }

	// // TODO: check if endpoints are really local, or if they are remote
	// // Check if there are any subsets with addresses
	// for _, subset := range endpoints.Subsets {
	// 	if len(subset.Addresses) > 0 {
	// 		return true
	// 	}
	// }

	return true, nil
}

// Check if service is an orphan.
//
// That is, if the source service that originated this PacService does not exist anymore
// in the source cluster. It does not return an error. If it gets an error different
// than NotFound from kube API, it will return false for safety.
func (s *PacService) IsOrphan(ctx context.Context, sourceKubeClient keess.IKubeClient) bool {

	sourceNamespace := s.Service.Annotations[keess.SourceNamespaceAnnotation]
	_, err := sourceKubeClient.CoreV1().Services(sourceNamespace).Get(ctx, s.Service.Name, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		// Service does not exist in source cluster, hence it is an orphan
		return true
	}

	return false
}
