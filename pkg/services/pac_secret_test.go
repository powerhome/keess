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
		Secret:  originalSecret,
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

	sourceSecret := core.Secret{
		ObjectMeta: v1.ObjectMeta{
			ResourceVersion: "1",
			Namespace:       "source-ns",
		},
		Data: map[string][]byte{
			"data": []byte("source-data"),
		},
	}
	pacSecret := PacSecret{
		Secret:  sourceSecret,
		Cluster: "source-cluster",
	}
	expectedRemoteSecret := core.Secret{
		ObjectMeta: v1.ObjectMeta{
			Labels: map[string]string{
				ManagedLabelSelector: "true",
			},
			Annotations: map[string]string{
				SourceResourceVersionAnnotation: "1",
				SourceClusterAnnotation:         "source-cluster",
				SourceNamespaceAnnotation:       "source-ns",
			},
		},
		Data: map[string][]byte{
			"data": []byte("source-data"),
		},
	}

	tests := []struct {
		name     string
		remote   core.Secret
		expected bool
	}{
		{
			name: "Different ResourceVersion",
			remote: func() core.Secret {
				s := expectedRemoteSecret.DeepCopy()
				s.ObjectMeta.Annotations[SourceResourceVersionAnnotation] = "2"

				return *s
			}(),
			expected: true,
		},
		{
			name: "ManagedLabelSelector not true",
			remote: func() core.Secret {
				s := expectedRemoteSecret.DeepCopy()
				s.ObjectMeta.Labels[ManagedLabelSelector] = "false"

				return *s
			}(),
			expected: true,
		},
		{
			name: "Different SourceClusterAnnotation",
			remote: func() core.Secret {
				s := expectedRemoteSecret.DeepCopy()
				s.ObjectMeta.Annotations[SourceClusterAnnotation] = "not-source-cluster"

				return *s
			}(),
			expected: true,
		},
		{
			name: "Different SourceNamespaceAnnotation",
			remote: func() core.Secret {
				s := expectedRemoteSecret.DeepCopy()
				s.ObjectMeta.Annotations[SourceNamespaceAnnotation] = "not-source-ns"

				return *s
			}(),
			expected: true,
		},
		{
			name: "Different Data",
			remote: func() core.Secret {
				s := expectedRemoteSecret.DeepCopy()
				s.Data["data"] = []byte("not-source-data")

				return *s
			}(),
			expected: true,
		},
		{
			name:     "No Changes",
			remote:   expectedRemoteSecret,
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
