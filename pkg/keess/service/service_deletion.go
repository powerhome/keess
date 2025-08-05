package service

import (
	"context"
	"keess/pkg/keess"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Delete orphan services in the local Kubernetes cluster.
func (s *ServiceSynchronizer) deleteOrphans(ctx context.Context, pollInterval time.Duration) error {
	// Poll for managed services. They will be pushed to this channel.
	mngSvcChan, err := s.servicePoller.PollServices(ctx, v1.ListOptions{
		LabelSelector: keess.ManagedLabelSelector,
	}, pollInterval)

	if err != nil {
		s.logger.Error("Failed to list managed services: ", err)
		return err
	}

	go func() {
		for {
			select {
			case service, ok := <-mngSvcChan:
				if !ok {
					// Channel closed, stop the goroutine
					return
				}
				s.logger.Debugf("[Service][deleteOrphans] Processing service %s/%s", service.Service.Namespace, service.Service.Name)

				// Process the service
				sourceCluster := service.Service.Annotations[keess.SourceClusterAnnotation]
				sourceNamespace := service.Service.Annotations[keess.SourceNamespaceAnnotation]

				var remoteKubeClient keess.IKubeClient
				if sourceCluster == service.Cluster {
					// TODO: this case should be impossible for services, since there is no namespace sync for services
					remoteKubeClient = s.localKubeClient
				} else {

					if _, ok := s.remoteKubeClients[sourceCluster]; !ok {
						s.logger.Error("[Service][deleteOrphans] Remote client not found: ", sourceCluster)
						continue
					}

					remoteKubeClient = s.remoteKubeClients[sourceCluster]
				}
				// TODO: extract this to a function

				// Check if the service is orphan
				_, err := remoteKubeClient.CoreV1().Services(sourceNamespace).Get(ctx, service.Service.Name, v1.GetOptions{})

				// Delete the orphan service
				if errors.IsNotFound(err) {
					// Check if the service has local endpoints before deleting
					hasLocalEndpoints := s.hasLocalEndpoints(ctx, service.Service)
					if hasLocalEndpoints {
						s.logger.Infof("[Service][deleteOrphans] Service %s/%s has local endpoints, skipping deletion", service.Service.Namespace, service.Service.Name)
						continue
					}

					err := s.localKubeClient.CoreV1().Services(service.Service.Namespace).Delete(ctx, service.Service.Name, v1.DeleteOptions{})
					if err == nil {
						s.logger.Infof("[Service][deleteOrphans] Deleted orphan service %s/%s", service.Service.Namespace, service.Service.Name)
					} else {
						s.logger.Error("[Service][deleteOrphans] Failed to delete orphan service: ", err)
					}

					// Check if namespace is managed and empty
					if s.isNamespaceManaged(service.Service.Namespace) {
						s.checkAndDeleteEmptyNamespace(ctx, service.Service.Namespace)
					}
				} else if err != nil {
					s.logger.Error("[Service][deleteOrphans] Failed to get remote service: ", err)
				}

			case <-ctx.Done():
				s.logger.Warn("[Service][deleteOrphans] Orphan deletion stopped by context cancellation.")
				return
			}
		}
	}()

	return nil
}

// Check if a service has local endpoints
func (s *ServiceSynchronizer) hasLocalEndpoints(ctx context.Context, service corev1.Service) bool {
	// Check if the service has a non-empty selector
	if len(service.Spec.Selector) == 0 {
		return false
	}

	// Get endpoints for the service
	endpoints, err := s.localKubeClient.CoreV1().Endpoints(service.Namespace).Get(ctx, service.Name, v1.GetOptions{})
	if err != nil {
		s.logger.Debugf("[Service][hasLocalEndpoints] Failed to get endpoints for service %s/%s: %v", service.Namespace, service.Name, err)
		return false
	}

	// TODO: check if endpoints are really local, or if they are remote
	// Check if there are any subsets with addresses
	for _, subset := range endpoints.Subsets {
		if len(subset.Addresses) > 0 {
			return true
		}
	}

	return false
}

// Check if namespace is managed by keess
func (s *ServiceSynchronizer) isNamespaceManaged(namespace string) bool {
	// This would need to be implemented based on how keess tracks managed namespaces
	// For now, we'll assume all namespaces are potentially managed
	// TODO: implement this
	return true
}

// Check and delete empty namespace
func (s *ServiceSynchronizer) checkAndDeleteEmptyNamespace(ctx context.Context, namespace string) {
	// TODO: rewrite this to check for all cluster resources. Also, decouple check from deletion.
	// List all resources in the namespace
	services, err := s.localKubeClient.CoreV1().Services(namespace).List(ctx, v1.ListOptions{})
	if err != nil {
		s.logger.Error("[Service][checkAndDeleteEmptyNamespace] Failed to list services in namespace: ", err)
		return
	}

	// If namespace is empty (only managed services), delete it
	if len(services.Items) == 0 {
		err := s.localKubeClient.CoreV1().Namespaces().Delete(ctx, namespace, v1.DeleteOptions{})
		if err == nil {
			s.logger.Infof("[Service][checkAndDeleteEmptyNamespace] Deleted empty namespace %s", namespace)
		} else {
			s.logger.Error("[Service][checkAndDeleteEmptyNamespace] Failed to delete namespace: ", err)
		}
	}
}
