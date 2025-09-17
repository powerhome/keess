package keess

import (
	"context"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

	return false
}

// IsOrphan checks if Secret is an orphan.
//
// That is, if the source Secret that originated this PacSecret does not exist anymore
// in the source cluster (or exists but lost the keess sync label). It does not return
// an error. If it can't determine if the Secret exists or not it will return false for
// safety.
func (s *PacSecret) IsOrphan(ctx context.Context, sourceKubeClient IKubeClient) bool {

	sourceNamespace := s.Secret.Annotations[SourceNamespaceAnnotation]

	// list all synced secrets in sourceNamespace
	secretList, err := sourceKubeClient.CoreV1().Secrets(sourceNamespace).List(ctx, metav1.ListOptions{
		LabelSelector: LabelSelector,
	})
	if err != nil {
		// Some error occurred while listing secrets. Assume secret is not orphan, for safety
		return false
	}

	// Check if the secret exists in the list
	for _, secret := range secretList.Items {
		if secret.Name == s.Secret.Name {
			// Secret exists in source cluster, not orphan
			return false
		}
	}

	// We listed the secrets in the source namespace without any errors
	// And s.Secret.Name is NOT among the returned secrets
	// So it's safe to say the secret is orphaned
	return true
}
