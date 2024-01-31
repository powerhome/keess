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

func TestPollConfigMaps(t *testing.T) {
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

	// Create a ConfigMapPoller
	poller := &ConfigMapPoller{
		KubeClient: client,
		Logger:     l,
	}

	// Create a context
	ctx := context.Background()

	// Create a ListOptions
	opts := metav1.ListOptions{}

	// Call the PollConfigMaps method
	configMapsChan, err := poller.PollConfigMaps(ctx, opts, time.Second)
	if err != nil {
		t.Fatalf("Failed to start configMap poller: %v", err)
	}

	// Create a configMap
	ns := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
		},
	}

	// Add the configMap to the fake client
	_, err = client.CoreV1().ConfigMaps(metav1.NamespaceAll).Create(ctx, ns, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create configMap: %v", err)
	}

	// Wait for the configMap to be received on the channel
	time.Sleep(time.Second * 2)

	// Check if the configMap is received on the channel
	select {
	case receivedNs := <-configMapsChan:
		if receivedNs.ConfigMap.Name != ns.Name {
			t.Errorf("Expected configMap name to be '%s', got '%s'", ns.Name, receivedNs.ConfigMap.Name)
		}
	case <-time.After(time.Second):
		t.Error("Expected to receive a configMap on the channel")
	}

	// Update the configMap
	ns.Annotations = map[string]string{"test": "annotation"}
	_, err = client.CoreV1().ConfigMaps(metav1.NamespaceAll).Update(ctx, ns, metav1.UpdateOptions{})
	if err != nil {
		t.Fatalf("Failed to update configMap: %v", err)
	}

	// Wait for the updated configMap to be received on the channel
	time.Sleep(time.Second * 2)

	// Check if the updated configMap is received on the channel
	select {
	case receivedNs := <-configMapsChan:
		if !reflect.DeepEqual(receivedNs.ConfigMap.Annotations, ns.Annotations) {
			t.Errorf("Expected configMap annotations to be '%v', got '%v'", ns.Annotations, receivedNs.ConfigMap.Annotations)
		}
	case <-time.After(time.Second):
		t.Error("Expected to receive an updated configMap on the channel")
	}

	// Delete the configMap
	err = client.CoreV1().ConfigMaps(metav1.NamespaceAll).Delete(ctx, ns.Name, metav1.DeleteOptions{})
	if err != nil {
		t.Fatalf("Failed to delete configMap: %v", err)
	}

	// Wait for the configMap to be removed from the poller
	time.Sleep(time.Second * 2)

	// Check if the configMap is removed from the poller
	configMaps, err := client.CoreV1().ConfigMaps(metav1.NamespaceAll).List(ctx, opts)
	if err != nil {
		t.Fatalf("Failed to list configMaps: %v", err)
	}
	for _, ns := range configMaps.Items {
		if ns.Name == "test" {
			t.Error("Expected configMap to be deleted from the poller")
		}
	}
}
