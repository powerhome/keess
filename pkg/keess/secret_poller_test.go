package keess

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/fake"
)

func TestSecretPoller_PollSecrets(t *testing.T) {
	cluster := "test-cluster"
	mockKubeClient := &MockKubeClient{Clientset: fake.NewSimpleClientset()}
	logger, _ := zap.NewProduction()
	sugaredLogger := logger.Sugar()

	secretPoller := &SecretPoller{
		cluster:    cluster,
		kubeClient: mockKubeClient,
		logger:     sugaredLogger,
		startup:    true,
	}

	opts := metav1.ListOptions{}
	pollInterval := time.Second * 5

	ctx := context.Background()

	secretsChan, err := secretPoller.PollSecrets(ctx, opts, pollInterval)
	assert.NoError(t, err, "PollSecrets should not return an error")

	// Create test secrets
	testSecrets := []*v1.Secret{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "secret1",
				Namespace: "default",
			},
			Data: map[string][]byte{
				"key1": []byte("value1"),
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "secret2",
				Namespace: "default",
			},
			Data: map[string][]byte{
				"key2": []byte("value2"),
			},
		},
	}

	// Add test secrets to the fake clientset
	for _, secret := range testSecrets {
		_, err := mockKubeClient.CoreV1().Secrets(secret.Namespace).Create(ctx, secret, metav1.CreateOptions{})
		assert.NoError(t, err, "Failed to create test secret")
	}

	receivedSecrets := make(map[string]bool)
	time.Sleep(2 * time.Second)

	// Verify that the secrets are received on the channel
	go func() {
		for secret := range secretsChan {
			receivedSecrets[secret.Secret.Name] = true
		}
	}()

	// Wait for the secrets to be received
	for {
		if len(receivedSecrets) == len(testSecrets) {
			break
		}
		time.Sleep(1 * time.Second)
	}

	// Verify that all test secrets are received
	for _, secret := range testSecrets {
		assert.True(t, receivedSecrets[secret.Name], "Test secret not received")
	}

	ctx.Done()
}

func TestSecretPoller_PollSecrets_ErrorRecovery(t *testing.T) {
	cluster := "test-cluster"

	// Create a client that will fail the first 2 List operations, then succeed
	fakeClient := fake.NewSimpleClientset()
	mockKubeClient := NewErrorInjectingMockKubeClient(fakeClient, 2)

	logger := zap.NewNop().Sugar()

	secretPoller := &SecretPoller{
		cluster:    cluster,
		kubeClient: mockKubeClient,
		logger:     logger,
		startup:    true,
	}

	opts := metav1.ListOptions{}
	pollInterval := 500 * time.Millisecond

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Create test secret
	testSecret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-secret",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"key": []byte("value"),
		},
	}

	_, err := fakeClient.CoreV1().Secrets(testSecret.Namespace).Create(ctx, testSecret, metav1.CreateOptions{})
	assert.NoError(t, err, "Failed to create test secret")

	secretsChan, err := secretPoller.PollSecrets(ctx, opts, pollInterval)
	assert.NoError(t, err, "PollSecrets should not return an error")

	receivedSecrets := make(map[string]bool)
	done := make(chan bool)

	// Collect secrets from the channel
	go func() {
		for secret := range secretsChan {
			receivedSecrets[secret.Secret.Name] = true
		}
		done <- true
	}()

	// Wait for multiple poll cycles (enough to go through errors and success)
	// First poll: startup (immediate) - will fail (error 1)
	// Second poll: after 500ms - will fail (error 2)
	// Third poll: after 500ms - will succeed
	time.Sleep(2 * time.Second)

	// Cancel context to stop polling
	cancel()

	// Wait for channel to close
	select {
	case <-done:
		// Channel closed successfully
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for channel to close")
	}

	// Verify that the goroutine survived the errors and successfully polled
	assert.True(t, receivedSecrets[testSecret.Name], "Secret should be received after error recovery")
}

// GroupVersionKind is a mock method for testing purposes
func (ps *PacSecret) GroupVersionKind() schema.GroupVersionKind {
	return schema.GroupVersionKind{}
}
