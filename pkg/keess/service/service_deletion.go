package service

import (
	"context"
	"fmt"
	"keess/pkg/keess"
	"time"

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
					s.logger.Info("[Service][deleteOrphans] Orphan deletion stopped by channel closure.")
					return
				}

				err := s.proccessServiceDeleteOrphan(ctx, service)
				if err != nil {
					s.logger.Error(err) // err message already contains context
				}

			case <-ctx.Done():
				s.logger.Warn("[Service][deleteOrphans] Orphan deletion stopped by context cancellation.")
				return
			}
		}
	}()

	return nil
}

// Process the service for deletion if it is an orphan.
func (s *ServiceSynchronizer) proccessServiceDeleteOrphan(ctx context.Context, svc PacService) error {
	//TODO: add delete toggle
	sourceKubeClient, err := s.getSourceKubeClient(svc)
	if err != nil {
		return fmt.Errorf("[Service][processServiceDeleteOrphan] failed to get source kube client: %w", err)
	}

	if !svc.IsOrphan(ctx, sourceKubeClient) {
		s.logger.Debugf("[Service][processServiceDeleteOrphan] Skipping service %s/%s: NOT an orphan", svc.Service.Namespace, svc.Service.Name)
		return nil
	}
	s.logger.Infof("[Service][processServiceDeleteOrphan] Found orphan service %s/%s", svc.Service.Namespace, svc.Service.Name)

	hasLE, err := svc.HasLocalEndpoints(ctx, s.localKubeClient)
	if err != nil {
		return fmt.Errorf("[Service][processServiceDeleteOrphan] failed to check local endpoints: %w", err)
	}

	if hasLE {
		s.logger.Debugf("[Service][processServiceDeleteOrphan] service %s/%s has local endpoints, skipping deletion", svc.Service.Namespace, svc.Service.Name)
		return nil
	}
	s.logger.Debugf("[Service][processServiceDeleteOrphan] orphan service %s/%s is safe for deletion", svc.Service.Namespace, svc.Service.Name)

	err = s.localKubeClient.CoreV1().Services(svc.Service.Namespace).Delete(ctx, svc.Service.Name, v1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("[Service][processServiceDeleteOrphan] failed to delete orphan service: %w", err)
	}
	s.logger.Infof("[Service][processServiceDeleteOrphan] Deleted orphan service %s/%s", svc.Service.Namespace, svc.Service.Name)

	// Now let's check if the namespace should be deleted as well
	if !s.isNamespaceManaged(svc.Service.Namespace) {
		s.logger.Debugf("[Service][deleteOrphans] namespace %s is not managed, skipping deletion", svc.Service.Namespace)
		return nil
	}

	if !s.isNamespaceEmpty(ctx, svc.Service.Namespace) {
		s.logger.Debugf("[Service][deleteOrphans] namespace %s is not empty, skipping deletion", svc.Service.Namespace)
		return nil
	}

	s.logger.Debugf("[Service][deleteOrphans] managed namespace %s is safe for deletion", svc.Service.Namespace)
	err = s.localKubeClient.CoreV1().Namespaces().Delete(ctx, svc.Service.Namespace, v1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("[Service][deleteOrphans] failed to delete managed namespace: %w", err)
	}
	s.logger.Infof("[Service][deleteOrphans] Deleted managed namespace %s", svc.Service.Namespace)

	return nil
}

// Get the kube client for the source cluster
func (s *ServiceSynchronizer) getSourceKubeClient(svc PacService) (keess.IKubeClient, error) {

	sourceCluster := svc.Service.Annotations[keess.SourceClusterAnnotation]
	if sourceCluster == svc.Cluster {
		return nil, fmt.Errorf("source cluster is the same as local cluster: %s", sourceCluster)
	}

	if _, ok := s.remoteKubeClients[sourceCluster]; !ok {
		return nil, fmt.Errorf("remote client not found: %s", sourceCluster)
	}

	return s.remoteKubeClients[sourceCluster], nil
}

// Check if namespace is managed by keess
//
// It does not return an error. If it gets an error from kube API, it will return false
// for safety.
func (s *ServiceSynchronizer) isNamespaceManaged(namespace string) bool {
	ns, err := s.localKubeClient.CoreV1().Namespaces().Get(context.TODO(), namespace, v1.GetOptions{})
	if err != nil {
		// assume false to avoid deleting non-managed namespaces
		return false
	}

	value, ok := ns.Labels[keess.ManagedLabelSelector]
	if !ok {
		return false
	}

	return value == "true"
}

// Check if namespace is Empty
// This is a heavy operation
func (s *ServiceSynchronizer) isNamespaceEmpty(ctx context.Context, namespace string) bool {
	// TODO: implement this
	return false
}