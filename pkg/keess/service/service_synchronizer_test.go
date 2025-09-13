package service

import (
	"context"
	"testing"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"keess/pkg/keess"
)

// Helper function to create a basic test service with configurable metadata
func createTestService(name, namespace string, labels map[string]string, annotations map[string]string, serviceType corev1.ServiceType) corev1.Service {
	return corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   namespace,
			Labels:      labels,
			Annotations: annotations,
		},
		Spec: corev1.ServiceSpec{
			Type: serviceType,
			Ports: []corev1.ServicePort{
				{
					Name:     "http",
					Port:     80,
					Protocol: corev1.ProtocolTCP,
				},
			},
		},
	}
}

// Helper function to create a ServiceSynchronizer with mock clients
func createTestSynchronizer() (*ServiceSynchronizer, map[string]keess.IKubeClient) {
	logger := zap.NewNop().Sugar()
	localClient := &keess.MockKubeClient{Clientset: fake.NewSimpleClientset()}
	remoteClients := map[string]keess.IKubeClient{
		"cluster1": &keess.MockKubeClient{Clientset: fake.NewSimpleClientset()},
		"cluster2": &keess.MockKubeClient{Clientset: fake.NewSimpleClientset()},
	}

	synchronizer := NewServiceSynchronizer(
		localClient,
		remoteClients,
		nil, // servicePoller not needed for this test
		nil, // namespacePoller not needed for this test
		logger,
	)

	return synchronizer, remoteClients
}

// Helper function to create a ServiceSynchronizer with pre-existing namespaces
func createTestSynchronizerWithNamespaces(namespace string) (*ServiceSynchronizer, map[string]keess.IKubeClient) {
	logger := zap.NewNop().Sugar()
	localClient := &keess.MockKubeClient{Clientset: fake.NewSimpleClientset()}

	// Create remote clients with the target namespace already existing
	remoteClient1 := fake.NewSimpleClientset()
	remoteClient2 := fake.NewSimpleClientset()

	// Create the namespace in remote clusters
	testNamespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace,
		},
	}
	remoteClient1.CoreV1().Namespaces().Create(context.Background(), testNamespace, metav1.CreateOptions{})
	remoteClient2.CoreV1().Namespaces().Create(context.Background(), testNamespace, metav1.CreateOptions{})

	remoteClients := map[string]keess.IKubeClient{
		"cluster1": &keess.MockKubeClient{Clientset: remoteClient1},
		"cluster2": &keess.MockKubeClient{Clientset: remoteClient2},
	}

	synchronizer := NewServiceSynchronizer(
		localClient,
		remoteClients,
		nil, // servicePoller not needed for this test
		nil, // namespacePoller not needed for this test
		logger,
	)

	return synchronizer, remoteClients
}

// Helper function to verify that no services were synced to remote clusters
func verifyNoServicesSynced(t *testing.T, remoteClients map[string]keess.IKubeClient, namespace string) {
	for clusterName, client := range remoteClients {
		services, err := client.CoreV1().Services(namespace).List(context.Background(), metav1.ListOptions{})
		if err != nil {
			t.Errorf("Failed to list services in cluster %s: %v", clusterName, err)
		}
		if len(services.Items) != 0 {
			t.Errorf("Expected no services in cluster %s, but found %d", clusterName, len(services.Items))
		}
	}
}

// Helper function to test invalid sync cases
func testInvalidSyncCase(t *testing.T, testName string, service corev1.Service) {
	pacService := PacService{
		Cluster: "source-cluster",
		Service: service,
	}

	synchronizer, remoteClients := createTestSynchronizer()

	// Call sync - should return nil (no error) but skip processing
	err := synchronizer.sync(context.Background(), pacService)
	if err != nil {
		t.Errorf("[%s] Expected no error, got %v", testName, err)
	}

	// Verify service was not synced to remote clusters
	verifyNoServicesSynced(t, remoteClients, service.Namespace)
}

func TestServiceSynchronizer_Sync_InvalidSyncMode(t *testing.T) {
	service := createTestService(
		"test-service",
		"test-namespace",
		map[string]string{
			keess.LabelSelector: "namespace", // Not "cluster"
		},
		map[string]string{
			keess.ClusterAnnotation: "cluster1,cluster2",
		},
		corev1.ServiceTypeClusterIP,
	)

	testInvalidSyncCase(t, "InvalidSyncMode", service)
}

func TestServiceSynchronizer_Sync_MissingLabelSelector(t *testing.T) {
	service := createTestService(
		"test-service",
		"test-namespace",
		map[string]string{}, // No sync label
		map[string]string{
			keess.ClusterAnnotation: "cluster1,cluster2",
		},
		corev1.ServiceTypeClusterIP,
	)

	testInvalidSyncCase(t, "MissingLabelSelector", service)
}

func TestServiceSynchronizer_Sync_MissingClustersAnnotation(t *testing.T) {
	service := createTestService(
		"test-service",
		"test-namespace",
		map[string]string{
			keess.LabelSelector: "cluster",
		},
		map[string]string{}, // No clusters annotation
		corev1.ServiceTypeClusterIP,
	)

	testInvalidSyncCase(t, "MissingClustersAnnotation", service)
}

func TestServiceSynchronizer_Sync_NonClusterIPService(t *testing.T) {
	service := createTestService(
		"test-service",
		"test-namespace",
		map[string]string{
			keess.LabelSelector: "cluster",
		},
		map[string]string{
			keess.ClusterAnnotation: "cluster1,cluster2",
		},
		corev1.ServiceTypeNodePort, // Not ClusterIP
	)

	testInvalidSyncCase(t, "NonClusterIPService", service)
}

func TestServiceSynchronizer_Sync_ValidService(t *testing.T) {
	service := createTestService(
		"test-service",
		"test-namespace",
		map[string]string{
			keess.LabelSelector: "cluster",
		},
		map[string]string{
			keess.ClusterAnnotation: "cluster1,cluster2",
		},
		corev1.ServiceTypeClusterIP,
	)

	pacService := PacService{
		Cluster: "source-cluster",
		Service: service,
	}

	synchronizer, remoteClients := createTestSynchronizerWithNamespaces("test-namespace")

	// Call sync - should successfully sync to remote clusters
	err := synchronizer.sync(context.Background(), pacService)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Verify service was synced to remote clusters
	for clusterName, client := range remoteClients {
		services, err := client.CoreV1().Services("test-namespace").List(context.Background(), metav1.ListOptions{})
		if err != nil {
			t.Errorf("Failed to list services in cluster %s: %v", clusterName, err)
		}
		if len(services.Items) != 1 {
			t.Errorf("Expected 1 service in cluster %s, but found %d", clusterName, len(services.Items))
		}
		if len(services.Items) > 0 {
			syncedService := services.Items[0]
			if syncedService.Name != "test-service" {
				t.Errorf("Expected service name 'test-service' in cluster %s, got %s", clusterName, syncedService.Name)
			}
			if syncedService.Labels[keess.ManagedLabelSelector] != "true" {
				t.Errorf("Expected managed label to be 'true' in cluster %s, got %s", clusterName, syncedService.Labels[keess.ManagedLabelSelector])
			}
		}
	}
}
