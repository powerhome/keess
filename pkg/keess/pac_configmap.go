package keess

import (
	"context"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

// IsOrphan checks if ConfigMap is an orphan.
//
// That is, if the source ConfigMap that originated this PacConfigMap does not exist anymore
// in the source cluster (or exists but lost the keess sync label). It does not return
// an error. If it can't determine if the ConfigMap exists or not it will return false for
// safety.
func (s *PacConfigMap) IsOrphan(ctx context.Context, sourceKubeClient IKubeClient) bool {

	sourceNamespace := s.ConfigMap.Annotations[SourceNamespaceAnnotation]

	// list all synced configMaps in sourceNamespace
	cmList, err := sourceKubeClient.CoreV1().ConfigMaps(sourceNamespace).List(ctx, metav1.ListOptions{
		LabelSelector: LabelSelector,
	})
	if err != nil {
		// Some error occurred while listing configMaps. Assume configMap is not orphan, for safety
		return false
	}

	// Check if the configMap exists in the list
	for _, cm := range cmList.Items {
		if cm.Name == s.ConfigMap.Name {
			// ConfigMap exists in source cluster, not orphan
			return false
		}
	}

	// We listed the configMaps in the source namespace without any errors
	// And s.ConfigMap.Name is NOT among the returned configMaps
	// So it's safe to say the configMap is orphaned
	return true
}
