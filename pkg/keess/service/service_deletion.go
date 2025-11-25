package service

import (
	"context"
	"fmt"
	"keess/pkg/keess"
	"keess/pkg/keess/metrics"
	"time"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// deleteOrphans deletes orphan services in the local Kubernetes cluster.
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
		s.logger.Debug("Service orphan deleter goroutine started")
		metrics.GoroutinesInactive.WithLabelValues("service").Dec()
		defer metrics.GoroutinesInactive.WithLabelValues("service").Inc()
		defer s.logger.Debug("Service orphan deleter goroutine stopped")

		for {
			select {
			case service, ok := <-mngSvcChan:
				if !ok {
					s.logger.Info("[Service][deleteOrphans] Orphan deletion stopped by channel closure.")
					return
				}

				err := s.processServiceDeleteOrphan(ctx, service)
				if err != nil {
					metrics.ErrorCount.Inc()
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

// processServiceDeleteOrphan processes the service for deletion if it is an orphan.
func (s *ServiceSynchronizer) processServiceDeleteOrphan(ctx context.Context, svc PacService) error {

	sourceKubeClient, err := s.getSourceKubeClient(svc)
	if err != nil {
		return fmt.Errorf("[Service][processServiceDeleteOrphan] failed to get source kube client: %w", err)
	}

	if !svc.IsOrphan(ctx, sourceKubeClient) {
		s.logger.Debugf("[Service][processServiceDeleteOrphan] Skipping service %s/%s: NOT an orphan", svc.Service.Namespace, svc.Service.Name)
		return nil
	}
	metrics.OrphansDetected.WithLabelValues("service").Inc()
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
	metrics.OrphansRemoved.WithLabelValues("service").Inc()
	s.logger.Infof("[Service][processServiceDeleteOrphan] Deleted orphan service %s/%s", svc.Service.Namespace, svc.Service.Name)

	// NOTE: we decided not to implement managed namespace deletion for now
	return nil
}

// getSourceKubeClient gets the kube client for the source cluster.
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
