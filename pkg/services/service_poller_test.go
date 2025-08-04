package services

import (
	"context"
	"testing"
	"time"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestServicePoller_PollServices(t *testing.T) {
	// Create a fake Kubernetes client
	fakeClient := fake.NewSimpleClientset()

	// Create test services
	testService1 := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-service-1",
			Namespace: "default",
			Labels: map[string]string{
				LabelSelector: "true",
			},
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name:     "http",
					Port:     80,
					Protocol: corev1.ProtocolTCP,
				},
			},
		},
	}

	testService2 := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-service-2",
			Namespace: "kube-system",
			Labels: map[string]string{
				LabelSelector: "true",
			},
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name:     "https",
					Port:     443,
					Protocol: corev1.ProtocolTCP,
				},
			},
		},
	}

	// Create the services in the fake client
	_, err := fakeClient.CoreV1().Services("default").Create(context.Background(), testService1, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create test service 1: %v", err)
	}

	_, err = fakeClient.CoreV1().Services("kube-system").Create(context.Background(), testService2, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create test service 2: %v", err)
	}

	// Create a logger
	logger := zap.NewNop().Sugar()

	// Create a mock client that implements IKubeClient
	mockClient := &mockKubeClient{Clientset: fakeClient}

	// Create a service poller
	poller := NewServicePoller("test-cluster", mockClient, logger)

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Start polling
	servicesChan, err := poller.PollServices(ctx, metav1.ListOptions{
		LabelSelector: LabelSelector,
	}, 1*time.Second)

	if err != nil {
		t.Fatalf("Failed to start polling: %v", err)
	}

	// Collect services from the channel
	var services []PacService
	for {
		select {
		case service, ok := <-servicesChan:
			if !ok {
				// Channel closed
				goto done
			}
			services = append(services, service)
		case <-ctx.Done():
			goto done
		}
	}

done:
	// Check that our test services are present (there might be other default services)
	serviceNames := make(map[string]bool)
	for _, service := range services {
		serviceNames[service.Service.Name] = true
	}

	if !serviceNames["test-service-1"] {
		t.Error("Expected to find test-service-1")
	}

	if !serviceNames["test-service-2"] {
		t.Error("Expected to find test-service-2")
	}

	// Check that all services have the correct cluster
	for _, service := range services {
		if service.Cluster != "test-cluster" {
			t.Errorf("Expected cluster to be 'test-cluster', got %s", service.Cluster)
		}
	}
}
