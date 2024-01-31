package services

import (
	"context"
	"os"
	"reflect"
	"testing"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestPollNamespaces(t *testing.T) {
	// Create a fake client
	client := fake.NewSimpleClientset()

	atom := zap.NewAtomicLevel()
	atom.SetLevel(zapcore.DebugLevel)

	encoderCfg := zapcore.EncoderConfig{
		TimeKey:      "timestamp",
		EncodeTime:   zapcore.TimeEncoderOfLayout(time.RFC3339),
		EncodeLevel:  zapcore.CapitalLevelEncoder,
		EncodeCaller: zapcore.ShortCallerEncoder,
	}

	logger := zap.New(zapcore.NewCore(
		zapcore.NewJSONEncoder(encoderCfg),
		zapcore.Lock(os.Stdout),
		atom,
	))
	defer logger.Sync()

	l := logger.Sugar()

	// Create a NamespacePoller
	poller := &NamespacePoller{
		KubeClient: client,
		Logger:     l,
	}

	// Create a context
	ctx := context.Background()

	// Create a ListOptions
	opts := metav1.ListOptions{}

	// Call the PollNamespaces method
	namespacesChan, err := poller.PollNamespaces(ctx, opts, time.Second)
	if err != nil {
		t.Fatalf("Failed to start namespace poller: %v", err)
	}

	// Create a namespace
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
		},
	}

	// Add the namespace to the fake client
	_, err = client.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create namespace: %v", err)
	}

	// Wait for the namespace to be received on the channel
	time.Sleep(time.Second * 2)

	// Check if the namespace is received on the channel
	select {
	case receivedNs := <-namespacesChan:
		if receivedNs.Namespace.Name != ns.Name {
			t.Errorf("Expected namespace name to be '%s', got '%s'", ns.Name, receivedNs.Namespace.Name)
		}
	case <-time.After(time.Second):
		t.Error("Expected to receive a namespace on the channel")
	}

	// Update the namespace
	ns.Annotations = map[string]string{"test": "annotation"}
	_, err = client.CoreV1().Namespaces().Update(ctx, ns, metav1.UpdateOptions{})
	if err != nil {
		t.Fatalf("Failed to update namespace: %v", err)
	}

	// Wait for the updated namespace to be received on the channel
	time.Sleep(time.Second * 2)

	// Check if the updated namespace is received on the channel
	select {
	case receivedNs := <-namespacesChan:
		if !reflect.DeepEqual(receivedNs.Namespace.Annotations, ns.Annotations) {
			t.Errorf("Expected namespace annotations to be '%v', got '%v'", ns.Annotations, receivedNs.Namespace.Annotations)
		}
	case <-time.After(time.Second):
		t.Error("Expected to receive an updated namespace on the channel")
	}

	// Delete the namespace
	err = client.CoreV1().Namespaces().Delete(ctx, ns.Name, metav1.DeleteOptions{})
	if err != nil {
		t.Fatalf("Failed to delete namespace: %v", err)
	}

	// Wait for the namespace to be removed from the poller
	time.Sleep(time.Second * 2)

	// Check if the namespace is removed from the poller
	namespaces, err := client.CoreV1().Namespaces().List(ctx, opts)
	if err != nil {
		t.Fatalf("Failed to list namespaces: %v", err)
	}
	for _, ns := range namespaces.Items {
		if ns.Name == "test" {
			t.Error("Expected namespace to be deleted from the poller")
		}
	}
}
