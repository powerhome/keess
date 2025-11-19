package service

import (
	"context"
	"keess/pkg/keess"
	"keess/pkg/keess/metrics"
	"time"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// A struct that can synchronize services in a Kubernetes cluster.
type ServiceSynchronizer struct {
	localKubeClient   keess.IKubeClient
	remoteKubeClients map[string]keess.IKubeClient
	servicePoller     *ServicePoller
	namespacePoller   *keess.NamespacePoller
	logger            *zap.SugaredLogger
	Services          map[string]*PacService
}

// NewServiceSynchronizer creates a new ServiceSynchronizer.
func NewServiceSynchronizer(
	localKubeClient keess.IKubeClient,
	remoteKubeClients map[string]keess.IKubeClient,
	servicePoller *ServicePoller,
	namespacePoller *keess.NamespacePoller,
	logger *zap.SugaredLogger,
) *ServiceSynchronizer {
	return &ServiceSynchronizer{
		localKubeClient:   localKubeClient,
		remoteKubeClients: remoteKubeClients,
		servicePoller:     servicePoller,
		namespacePoller:   namespacePoller,
		logger:            logger,
		Services:          make(map[string]*PacService),
	}
}

// Start starts the service synchronizer.
func (s *ServiceSynchronizer) Start(ctx context.Context, pollInterval time.Duration, housekeepingInterval time.Duration) error {
	err := s.startSyncing(ctx, pollInterval)
	if err != nil {
		return err
	}

	err = s.deleteOrphans(ctx, housekeepingInterval)
	if err != nil {
		return err
	}

	return nil
}

// startSyncing starts syncing services (Poller and Synchronizer).
func (s *ServiceSynchronizer) startSyncing(ctx context.Context, pollInterval time.Duration) error {
	// Poll for service to be synced. They will be pushed to this channel.
	syncSvcChan, err := s.servicePoller.PollServices(ctx, v1.ListOptions{
		LabelSelector: keess.ClusterLabelSelector, // remember service sync does not make sense for namespace sync
	}, pollInterval)

	if err != nil {
		s.logger.Error("Failed to start service poller: ", err)
		return err
	}

	go func() {
		s.logger.Debug("Service synchronizer goroutine started")
		metrics.GoroutinesUp.WithLabelValues("service").Inc()
		defer metrics.GoroutinesUp.WithLabelValues("service").Dec()
		defer s.logger.Debug("Service synchronizer goroutine stopped")

		for {
			select {
			case service, ok := <-syncSvcChan:
				if !ok {
					// Channel closed, stop the goroutine
					s.logger.Warn("[Service][startSyncing] Service syncing stopped because services channel was closed.")
					return
				}
				s.logger.Debugf("[Service][Synchronizer] Processing service %s/%s", service.Service.Namespace, service.Service.Name)

				// Process the service
				err := s.sync(ctx, service)
				if err != nil {
					s.logger.Error("[Service][startSyncing] Failed to sync service: ", err)
				}

			case <-ctx.Done():
				s.logger.Warn("[Service][startSyncing] Service syncing stopped by context cancellation.")
				return
			}
		}
	}()

	return nil
}

// sync syncs a service to all remote clusters.
func (s *ServiceSynchronizer) sync(ctx context.Context, pacService PacService) error {

	// Check sync label is set to "cluster"
	syncMode, exists := pacService.Service.Labels[keess.LabelSelector]
	if !exists || syncMode != "cluster" {
		s.logger.Error("[Service][sync] Service sync requires cluster sync mode (", keess.LabelSelector, ": cluster), skipping sync for service: ", pacService.Service.Name)
		return nil
	}

	// Check if the service has the clusters annotation
	clustersAnnotation, exists := pacService.Service.Annotations[keess.ClusterAnnotation]
	if !exists {
		s.logger.Debug("[Service][sync] Service is marked for sync but does not have clusters annotation, skipping sync: ", pacService.Service.Name)
		return nil
	}

	if pacService.Service.Spec.Type != corev1.ServiceTypeClusterIP {
		s.logger.Errorf("[Service][sync] Only ClusterIP services are supported for sync, found %s, skipping sync for service: %s", pacService.Service.Spec.Type, pacService.Service.Name)
		return nil
	}

	// Parse the clusters
	clusters := keess.SplitAndTrim(clustersAnnotation, ",")

	// Sync to each cluster
	for _, cluster := range clusters {
		if cluster == pacService.Cluster {
			s.logger.Debug("[Service][sync] Service is marked for sync to its own cluster, skipping remote sync: ", pacService.Service.Name)
		} else {
			// Sync remotely
			err := s.syncRemote(ctx, pacService, cluster)
			if err != nil {
				s.logger.Error("[Service][sync] Failed to sync service remotely: ", err)
			}
		}
	}

	return nil
}

// syncRemote syncs a service to a remote cluster.
// This function assumes that the remote cluster client is already set up in the ServiceSynchronizer.
// It will create or update the service in the remote cluster.
func (s *ServiceSynchronizer) syncRemote(ctx context.Context, pacService PacService, cluster string) error {

	k, exists := s.remoteKubeClients[cluster]
	if !exists {
		s.logger.Error("[Service][syncRemote] Remote client not found: ", cluster)
		return nil
	}

	// Check if the namespace exists in the remote cluster
	_, err := k.CoreV1().Namespaces().Get(ctx, pacService.Service.Namespace, v1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			s.CreateNamespaceForService(ctx, k, cluster, pacService)
		} else {
			s.logger.Error("[Service][syncRemote] Failed to get namespace in remote cluster: ", err)
			return err
		}
	}

	err = s.createOrUpdate(ctx, k, pacService, cluster, pacService.Service.Namespace)
	if err != nil {
		s.logger.Error("[Service][syncRemote] Failed to create or update service: ", err)
	}

	return nil
}

// CreateNamespaceForService creates a namespace for the service.
func (s *ServiceSynchronizer) CreateNamespaceForService(ctx context.Context, k keess.IKubeClient, cluster string, svc PacService) error {
	ns := &corev1.Namespace{
		ObjectMeta: v1.ObjectMeta{
			Name: svc.Service.Namespace,
			Labels: map[string]string{
				keess.ManagedLabelSelector: "true",
			},
			Annotations: map[string]string{
				keess.SourceClusterAnnotation:   svc.Cluster,
				keess.SourceNamespaceAnnotation: svc.Service.Namespace,
			},
		},
	}

	_, err := k.CoreV1().Namespaces().Create(ctx, ns, v1.CreateOptions{})
	if err != nil {
		s.logger.Error("[Service][syncRemote] Failed to create namespace in remote cluster: ", err)
		return err
	}

	s.logger.Infof("[Service][syncRemote] Namespace %s created in cluster %s", svc.Service.Namespace, cluster)
	return nil
}

// createOrUpdate creates or updates a service.
func (s *ServiceSynchronizer) createOrUpdate(ctx context.Context, k keess.IKubeClient, pacSvc PacService, cluster string, ns string) error {
	// Prepare the service for the target namespace
	svc := pacSvc.Prepare(ns)

	// Check if the service already exists
	existingService, err := k.CoreV1().Services(ns).Get(ctx, svc.Name, v1.GetOptions{})

	if errors.IsNotFound(err) {
		// Service does not exist, create it
		_, err = k.CoreV1().Services(ns).Create(ctx, &svc, v1.CreateOptions{})
		if err == nil {
			s.logger.Infof("[Service][createOrUpdate] Created service %s/%s in cluster %s", ns, svc.Name, cluster)
		} else {
			s.logger.Error("[Service][createOrUpdate] Failed to create service: ", err)
		}
		return err
	} else if err != nil {
		s.logger.Error("[Service][createOrUpdate] Failed to get existing services: ", err)
		return err
	}

	// Service exist, check if we should manage it
	managed, ok := existingService.Labels[keess.ManagedLabelSelector]
	if !ok {
		s.logger.Warnf("[Service][createOrUpdate] Cluster %s already has unmanaged Service %s/%s, skipping create/update", cluster, ns, svc.Name)
		return nil
	}
	if managed != "true" {
		s.logger.Warnf("[Service][createOrUpdate] Service %s/%s on Cluster %s has management disabled, skipping create/update", ns, svc.Name, cluster)
		return nil
	}

	// Check if the service has changed
	if pacSvc.HasChanged(*existingService) {
		// Update the service
		svc.ResourceVersion = existingService.ResourceVersion
		_, err = k.CoreV1().Services(ns).Update(ctx, &svc, v1.UpdateOptions{})
		if err == nil {
			s.logger.Infof("[Service][createOrUpdate] Updated service %s/%s in cluster %s", ns, svc.Name, cluster)
		} else {
			s.logger.Error("[Service][createOrUpdate] Failed to update service: ", err)
		}
		return err
	}

	return nil
}
