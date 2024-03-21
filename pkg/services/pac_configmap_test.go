package services

import (
	"testing"

	"github.com/stretchr/testify/assert"
	core "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestConfigMapPrepare(t *testing.T) {
	originalConfigMap := core.ConfigMap{
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
	pacConfigMap := PacConfigMap{
		ConfigMap: originalConfigMap,
		Cluster:   "test-cluster",
	}

	newNamespace := "new-ns"
	preparedConfigMap := pacConfigMap.Prepare(newNamespace)

	// Check if the original ConfigMap was not modified
	assert.Equal(t, newNamespace, preparedConfigMap.Namespace, "Namespace should be updated")

	// Check if the UID was cleared
	assert.Empty(t, preparedConfigMap.UID, "UID should be empty")

	// Check if the Labels were correctly set
	assert.Equal(t, map[string]string{ManagedLabelSelector: "true"}, preparedConfigMap.Labels, "Labels should be correctly set")

	// Check if the Annotations were correctly set
	expectedAnnotations := map[string]string{
		SourceClusterAnnotation:         "test-cluster",
		SourceNamespaceAnnotation:       "original-ns",
		SourceResourceVersionAnnotation: "123",
	}
	assert.Equal(t, expectedAnnotations, preparedConfigMap.Annotations, "Annotations should be correctly set")

	// Check if the ResourceVersion was cleared
	assert.Empty(t, preparedConfigMap.ResourceVersion, "ResourceVersion should be empty")
}

func TestConfigMapHasChanged(t *testing.T) {

	localConfigMap := core.ConfigMap{
		ObjectMeta: v1.ObjectMeta{
			ResourceVersion: "1",
			Namespace:       "local-ns",
		},
	}
	pacConfigMap := PacConfigMap{
		ConfigMap: localConfigMap,
		Cluster:   "local-cluster",
	}

	tests := []struct {
		name     string
		remote   core.ConfigMap
		expected bool
	}{
		{
			name: "Different ResourceVersion",
			remote: core.ConfigMap{
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
			remote: core.ConfigMap{
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
			remote: core.ConfigMap{
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
			remote: core.ConfigMap{
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
			remote: core.ConfigMap{
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
			result := pacConfigMap.HasChanged(test.remote)
			assert.Equal(t, test.expected, result)
		})
	}
}
