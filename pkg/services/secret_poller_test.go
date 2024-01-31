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

func TestPollSecrets(t *testing.T) {
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

	// Create a SecretPoller
	poller := &SecretPoller{
		KubeClient: client,
		Logger:     l,
	}

	// Create a context
	ctx := context.Background()

	// Create a ListOptions
	opts := metav1.ListOptions{}

	// Call the PollSecrets method
	secretsChan, err := poller.PollSecrets(ctx, opts, time.Second)
	if err != nil {
		t.Fatalf("Failed to start secret poller: %v", err)
	}

	// Create a secret
	ns := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
		},
	}

	// Add the secret to the fake client
	_, err = client.CoreV1().Secrets(metav1.NamespaceAll).Create(ctx, ns, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create secret: %v", err)
	}

	// Wait for the secret to be received on the channel
	time.Sleep(time.Second * 2)

	// Check if the secret is received on the channel
	select {
	case receivedNs := <-secretsChan:
		if receivedNs.Secret.Name != ns.Name {
			t.Errorf("Expected secret name to be '%s', got '%s'", ns.Name, receivedNs.Secret.Name)
		}
	case <-time.After(time.Second):
		t.Error("Expected to receive a secret on the channel")
	}

	// Update the secret
	ns.Annotations = map[string]string{"test": "annotation"}
	_, err = client.CoreV1().Secrets(metav1.NamespaceAll).Update(ctx, ns, metav1.UpdateOptions{})
	if err != nil {
		t.Fatalf("Failed to update secret: %v", err)
	}

	// Wait for the updated secret to be received on the channel
	time.Sleep(time.Second * 2)

	// Check if the updated secret is received on the channel
	select {
	case receivedNs := <-secretsChan:
		if !reflect.DeepEqual(receivedNs.Secret.Annotations, ns.Annotations) {
			t.Errorf("Expected secret annotations to be '%v', got '%v'", ns.Annotations, receivedNs.Secret.Annotations)
		}
	case <-time.After(time.Second):
		t.Error("Expected to receive an updated secret on the channel")
	}

	// Delete the secret
	err = client.CoreV1().Secrets(metav1.NamespaceAll).Delete(ctx, ns.Name, metav1.DeleteOptions{})
	if err != nil {
		t.Fatalf("Failed to delete secret: %v", err)
	}

	// Wait for the secret to be removed from the poller
	time.Sleep(time.Second * 2)

	// Check if the secret is removed from the poller
	secrets, err := client.CoreV1().Secrets(metav1.NamespaceAll).List(ctx, opts)
	if err != nil {
		t.Fatalf("Failed to list secrets: %v", err)
	}
	for _, ns := range secrets.Items {
		if ns.Name == "test" {
			t.Error("Expected secret to be deleted from the poller")
		}
	}
}
