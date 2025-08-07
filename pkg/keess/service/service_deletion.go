package service

import (
	"context"
	"errors"
	"fmt"
	"keess/pkg/keess"
	"strings"
	"time"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
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

// isNamespaceManaged checks if namespace is managed by keess
//
// It does not return an error. If it gets an error from kube API, it will return false
// for safety.
func (s *ServiceSynchronizer) isNamespaceManaged(namespace string) bool {
	ns, err := s.localKubeClient.CoreV1().Namespaces().Get(context.TODO(), namespace, v1.GetOptions{})
	if err != nil {
		// assume false to avoid deleting non-managed namespaces
		s.logger.Errorf("[Service][isNamespaceManaged] Got error while checking namespace %s: %v", namespace, err)
		return false
	}

	value, ok := ns.Labels[keess.ManagedLabelSelector]
	if !ok {
		s.logger.Debugf("[Service][isNamespaceManaged] Namespace %s has no managed label", namespace)
		return false
	}

	return value == "true"
}

// isNamespaceEmpty checks if namespace is Empty
// This is a heavy operation. It fetches every resource GVK (Group/Version/Kind) from
// Kubernetes API, and for every single one of them it checks if there are any present
// in the namespace.
// It only returns a boolean value, not an error. If it gets any error, it will log the
// error and assume that the namespace is not empty for safety and return false.
func (s *ServiceSynchronizer) isNamespaceEmpty(ctx context.Context, namespace string) bool {
	discoveryClient := s.localKubeClient.Discovery()
	if discoveryClient == nil {
		s.logger.Error("[Service][isNamespaceEmpty] Cannot get kubernetes Discovery client, assuming namespace is not empty for safety")
		return false
	}

	dynClient := s.localKubeClient.Dynamic()
	if dynClient == nil {
		s.logger.Error("[Service][isNamespaceEmpty] Cannot get kubernetes Dynamic client, assuming namespace is not empty for safety")
		return false
	}

	// Get all API resources from the discovery API
	apiResourceLists, err := discoveryClient.ServerPreferredResources()
	if err != nil {
		// If we can't discover resources, assume namespace is not empty for safety
		s.logger.Warnf("[Service][isNamespaceEmpty] Failed to discover API resources: %v", err)
		return false
	}

	// Check each resource type for any instances in the namespace
	for _, apiResourceList := range apiResourceLists {
		if apiResourceList == nil {
			continue
		}

		// Parse the group version
		gv, err := schema.ParseGroupVersion(apiResourceList.GroupVersion)
		if err != nil {
			s.logger.Warnf("[Service][isNamespaceEmpty] Failed to parse group version %s: %v", apiResourceList.GroupVersion, err)
			continue
		}

		for _, resource := range apiResourceList.APIResources {

			// Skip some resource types that are typically not user-managed or don't affect "emptiness"
			if s.shouldSkipResource(resource) {
				s.logger.Debugf("[Service][isNamespaceEmpty] Skipping resource %s/%s", gv.String(), resource.Name)
				continue
			}

			// Check if there are any instances of this resource in the namespace
			hasResources, err := s.hasResourcesInNamespace(ctx, dynClient, gv, resource, namespace)
			if err != nil {
				s.logger.Errorf("[Service][isNamespaceEmpty] Failed to check %s/%s in namespace %s: %v", gv.String(), resource.Name, namespace, err)
				// If we can't check, assume namespace is not empty for safety
				return false
			}

			if hasResources {
				s.logger.Debugf("[Service][isNamespaceEmpty] Found %s/%s resources in namespace %s", gv.String(), resource.Name, namespace)
				return false
			}
		}
	}

	s.logger.Debugf("[Service][isNamespaceEmpty] No resources found in namespace %s", namespace)
	return true
}

// shouldSkipResource determines if we should skip checking certain resource types
// These are typically system-managed resources that shouldn't prevent namespace deletion
// Also skips non-namespaced resources and subresources (those with '/')
func (s *ServiceSynchronizer) shouldSkipResource(resource v1.APIResource) bool {
	skipResources := []string{
		"events",                     // Events are ephemeral
		"componentstatuses",          // System component status
		"bindings",                   // Pod bindings
		"localsubjectaccessreviews",  // RBAC checks
		"selfsubjectaccessreviews",   // RBAC checks
		"selfsubjectrulesreviews",    // RBAC checks
		"subjectaccessreviews",       // RBAC checks
		"tokenreviews",               // Authentication tokens
		"certificatesigningrequests", // CSRs are typically cleaned up
		"serviceaccounts",            // Some service accounts are managed by Kubernetes itself, and we can accept to lose one if it's the only thing left on a namespace
	}

	// Skip subresources (they contain '/')
	if strings.Contains(resource.Name, "/") {
		return true
	}

	// Only check namespaced resources
	if !resource.Namespaced {
		return true
	}

	for _, skip := range skipResources {
		if resource.Name == skip {
			return true
		}
	}
	return false
}

// hasResourcesInNamespace checks if there are any resources of the given type in the namespace
func (s *ServiceSynchronizer) hasResourcesInNamespace(ctx context.Context, dynClient dynamic.Interface, gv schema.GroupVersion, resource v1.APIResource, namespace string) (bool, error) {
	s.logger.Debugf("[Service][hasResourcesInNamespace] Checking for %s/%s in namespace %s", gv.String(), resource.Name, namespace)

	// Build the resource identifier
	gvr := schema.GroupVersionResource{
		Group:    gv.Group,
		Version:  gv.Version,
		Resource: resource.Name,
	}

	// Use dynamic client if available, fall back to specific CoreV1 methods
	if dynClient == nil {
		return true, errors.New("[Service][hasResourcesInNamespace] Got an empty dynamic client")
	}

	// Use dynamic client to list resources with a limit of 1
	result, err := dynClient.Resource(gvr).Namespace(namespace).List(ctx, v1.ListOptions{Limit: 1})
	if err != nil {
		// // If we get a 404, the resource type doesn't exist, so no resources
		// if strings.Contains(err.Error(), "404") || strings.Contains(err.Error(), "not found") {
		// 	return false, nil
		// }
		// If we can't list resources, assume namespace has resources for safety
		return true, err
	}

	return len(result.Items) > 0, nil
}

// // Fall back to CoreV1 for common resources if dynamic client not available
// if gv.Group == "" {
// 	switch resource.Name {
// 	case "pods":
// 		pods, err := s.localKubeClient.CoreV1().Pods(namespace).List(ctx, v1.ListOptions{Limit: 1})
// 		if err != nil {
// 			return false, err
// 		}
// 		return len(pods.Items) > 0, nil
// 	case "services":
// 		services, err := s.localKubeClient.CoreV1().Services(namespace).List(ctx, v1.ListOptions{Limit: 1})
// 		if err != nil {
// 			return false, err
// 		}
// 		return len(services.Items) > 0, nil
// 	case "configmaps":
// 		configmaps, err := s.localKubeClient.CoreV1().ConfigMaps(namespace).List(ctx, v1.ListOptions{Limit: 1})
// 		if err != nil {
// 			return false, err
// 		}
// 		return len(configmaps.Items) > 0, nil
// 	case "secrets":
// 		secrets, err := s.localKubeClient.CoreV1().Secrets(namespace).List(ctx, v1.ListOptions{Limit: 1})
// 		if err != nil {
// 			return false, err
// 		}
// 		return len(secrets.Items) > 0, nil
// 	case "persistentvolumeclaims":
// 		pvcs, err := s.localKubeClient.CoreV1().PersistentVolumeClaims(namespace).List(ctx, v1.ListOptions{Limit: 1})
// 		if err != nil {
// 			return false, err
// 		}
// 		return len(pvcs.Items) > 0, nil
// 	case "endpoints":
// 		endpoints, err := s.localKubeClient.CoreV1().Endpoints(namespace).List(ctx, v1.ListOptions{Limit: 1})
// 		if err != nil {
// 			return false, err
// 		}
// 		return len(endpoints.Items) > 0, nil
// 	case "serviceaccounts":
// 		serviceaccounts, err := s.localKubeClient.CoreV1().ServiceAccounts(namespace).List(ctx, v1.ListOptions{Limit: 1})
// 		if err != nil {
// 			return false, err
// 		}
// 		return len(serviceaccounts.Items) > 0, nil
// 	}
// }
