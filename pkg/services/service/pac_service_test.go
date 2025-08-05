package service

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"keess/pkg/services"
)

func TestPacService_Prepare(t *testing.T) {
	// Create a test service
	originalService := corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-service",
			Namespace: "source-namespace",
			Labels: map[string]string{
				"app": "test",
			},
			Annotations: map[string]string{
				services.CiliumGlobalServiceAnnotation: "true",
			},
			ResourceVersion: "123",
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				"app": "test",
			},
			Ports: []corev1.ServicePort{
				{
					Name:     "http",
					Port:     80,
					Protocol: corev1.ProtocolTCP,
				},
			},
		},
	}

	pacService := PacService{
		Cluster: "source-cluster",
		Service: originalService,
	}

	// Prepare the service for a target namespace
	preparedService := pacService.Prepare("target-namespace")

	// Verify the prepared service
	if preparedService.Namespace != "target-namespace" {
		t.Errorf("Expected namespace to be 'target-namespace', got %s", preparedService.Namespace)
	}

	if preparedService.Name != "test-service" {
		t.Errorf("Expected name to be 'test-service', got %s", preparedService.Name)
	}

	if preparedService.Labels[services.ManagedLabelSelector] != "true" {
		t.Errorf("Expected managed label to be 'true', got %s", preparedService.Labels[services.ManagedLabelSelector])
	}

	if preparedService.Annotations[services.SourceClusterAnnotation] != "source-cluster" {
		t.Errorf("Expected source cluster annotation to be 'source-cluster', got %s", preparedService.Annotations[services.SourceClusterAnnotation])
	}

	if preparedService.Annotations[services.SourceNamespaceAnnotation] != "source-namespace" {
		t.Errorf("Expected source namespace annotation to be 'source-namespace', got %s", preparedService.Annotations[services.SourceNamespaceAnnotation])
	}

	if preparedService.Annotations[services.SourceResourceVersionAnnotation] != "123" {
		t.Errorf("Expected source resource version annotation to be '123', got %s", preparedService.Annotations[services.SourceResourceVersionAnnotation])
	}

	if preparedService.Annotations[services.CiliumGlobalServiceAnnotation] != "true" {
		t.Errorf("Expected Cilium global annotation to be 'true', got %s", preparedService.Annotations[services.CiliumGlobalServiceAnnotation])
	}

	if preparedService.Annotations[services.CiliumSharedServiceAnnotation] != "false" {
		t.Errorf("Expected Cilium shared annotation to be 'false', got %s", preparedService.Annotations[services.CiliumSharedServiceAnnotation])
	}

	if len(preparedService.Spec.Selector) != 0 {
		t.Errorf("Expected selector to be empty, got %v", preparedService.Spec.Selector)
	}

	if preparedService.ResourceVersion != "" {
		t.Errorf("Expected resource version to be empty, got %s", preparedService.ResourceVersion)
	}
}

func TestPacService_HasChanged(t *testing.T) {
	originalService := corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "test-service",
			Namespace:       "source-namespace",
			ResourceVersion: "123",
		},
	}

	pacService := PacService{
		Cluster: "source-cluster",
		Service: originalService,
	}

	// Test case 1: Service has not changed
	unchangedService := corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-service",
			Namespace: "target-namespace",
			Labels: map[string]string{
				services.ManagedLabelSelector: "true",
			},
			Annotations: map[string]string{
				services.SourceClusterAnnotation:         "source-cluster",
				services.SourceNamespaceAnnotation:       "source-namespace",
				services.SourceResourceVersionAnnotation: "123",
				services.CiliumGlobalServiceAnnotation:   "true",
				services.CiliumSharedServiceAnnotation:   "false",
			},
		},
	}

	if pacService.HasChanged(unchangedService) {
		t.Error("Expected service to not have changed")
	}

	// Test case 2: Service has changed (different resource version)
	changedService := corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-service",
			Namespace: "target-namespace",
			Labels: map[string]string{
				services.ManagedLabelSelector: "true",
			},
			Annotations: map[string]string{
				services.SourceClusterAnnotation:         "source-cluster",
				services.SourceNamespaceAnnotation:       "source-namespace",
				services.SourceResourceVersionAnnotation: "456", // Different resource version
				services.CiliumGlobalServiceAnnotation:   "true",
				services.CiliumSharedServiceAnnotation:   "false",
			},
		},
	}

	if !pacService.HasChanged(changedService) {
		t.Error("Expected service to have changed")
	}
}
