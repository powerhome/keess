package services

import (
	"testing"

	"github.com/stretchr/testify/assert"
	core "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestPrepare(t *testing.T) {
	originalSecret := core.Secret{
		ObjectMeta: v1.ObjectMeta{
			Namespace:       "original-ns",
			UID:             "original-uid",
			ResourceVersion: "123",
			Annotations: map[string]string{
				"SomeOtherAnnotation": "value",
			},
			Labels: map[string]string{
				"SomeOtherLabel": "value",
			},
		},
	}
	pacSecret := PacSecret{
		Secret:  &originalSecret,
		Cluster: "test-cluster",
	}

	newNamespace := "new-ns"
	preparedSecret := pacSecret.Prepare(newNamespace)

	// Check if the original Secret was not modified
	assert.Equal(t, newNamespace, preparedSecret.Namespace, "Namespace should be updated")

	// Check if the UID was cleared
	assert.Empty(t, preparedSecret.UID, "UID should be empty")

	// Check if the Labels were correctly set
	assert.Equal(t, map[string]string{ManagedLabelSelector: "true"}, preparedSecret.Labels, "Labels should be correctly set")

	// Check if the Annotations were correctly set
	expectedAnnotations := map[string]string{
		SourceClusterAnnotation:         "test-cluster",
		SourceNamespaceAnnotation:       "original-ns",
		SourceResourceVersionAnnotation: "123",
	}
	assert.Equal(t, expectedAnnotations, preparedSecret.Annotations, "Annotations should be correctly set")

	// Check if the ResourceVersion was cleared
	assert.Empty(t, preparedSecret.ResourceVersion, "ResourceVersion should be empty")
}

func TestHasChanged(t *testing.T) {

	localSecret := &core.Secret{
		ObjectMeta: v1.ObjectMeta{
			ResourceVersion: "1",
			Namespace:       "local-ns",
		},
	}
	pacSecret := PacSecret{
		Secret:  localSecret,
		Cluster: "local-cluster",
	}

	tests := []struct {
		name     string
		remote   *core.Secret
		expected bool
	}{
		{
			name: "Different ResourceVersion",
			remote: &core.Secret{
				ObjectMeta: v1.ObjectMeta{
					Annotations: map[string]string{
						SourceResourceVersionAnnotation: "2",
					},
				},
			},
			expected: true,
		},
		{
			name: "ManagedLabelSelector not true",
			remote: &core.Secret{
				ObjectMeta: v1.ObjectMeta{
					Annotations: map[string]string{
						SourceResourceVersionAnnotation: "1",
					},
					Labels: map[string]string{
						ManagedLabelSelector: "false",
					},
				},
			},
			expected: true,
		},
		{
			name: "Different SourceClusterAnnotation",
			remote: &core.Secret{
				ObjectMeta: v1.ObjectMeta{
					Labels: map[string]string{
						ManagedLabelSelector: "true",
					},
					Annotations: map[string]string{
						SourceResourceVersionAnnotation: "1",
						SourceClusterAnnotation:         "remote-cluster",
					},
				},
			},
			expected: true,
		},
		{
			name: "Different SourceNamespaceAnnotation",
			remote: &core.Secret{
				ObjectMeta: v1.ObjectMeta{
					Labels: map[string]string{
						ManagedLabelSelector: "true",
					},
					Annotations: map[string]string{
						SourceResourceVersionAnnotation: "1",
						SourceClusterAnnotation:         "local-cluster",
						SourceNamespaceAnnotation:       "remote-ns",
					},
				},
			},
			expected: true,
		},
		{
			name: "No Changes",
			remote: &core.Secret{
				ObjectMeta: v1.ObjectMeta{
					Annotations: map[string]string{
						SourceResourceVersionAnnotation: "1",
						SourceClusterAnnotation:         "local-cluster",
						SourceNamespaceAnnotation:       "local-ns",
					},
					Labels: map[string]string{
						ManagedLabelSelector: "true",
					},
				},
			},
			expected: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := pacSecret.HasChanged(test.remote)
			assert.Equal(t, test.expected, result)
		})
	}
}
