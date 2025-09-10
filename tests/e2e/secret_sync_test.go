package e2e_test

import (
	"context"
	"fmt"
	"reflect"

	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

var (
	exampleFile = filepath.Join("..", "..", "examples", "test-secret-sync-example.yaml")
	// secretName and namespace must match the example file
	secretName      = "app-secret"
	secretNamespace = "test-keess"
)

// getSecret gets a Secret using kubernetes client.
func getSecret(ctx context.Context, client kubernetes.Interface, name, namespace string) (*corev1.Secret, error) {
	return client.CoreV1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{})
}

// secretIsNotFound checks if Secret is not found (shortcut).
func secretIsNotFound(ctx context.Context, client kubernetes.Interface, name, namespace string) bool {
	_, err := client.CoreV1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{})
	return errors.IsNotFound(err)
}

var _ = Describe("Secret Sync", Label("secret"), func() {
	Context("On Cluster mode", func() {

		BeforeEach(func(ctx SpecContext) {
			By("Ensuring clean start by recreating test namespace")
			deleteNamespaceOnAll(ctx, secretNamespace, true)
			createNamespaceOnAll(ctx, secretNamespace)
		}, NodeTimeout(shortT))

		AfterEach(func(ctx SpecContext) {
			By("Cleaning up by removing test namespace")
			deleteNamespaceOnAll(ctx, secretNamespace, false)
		}, NodeTimeout(shortT))

		When("an annotated Secret is created in the source cluster", func() {

			It("it should be synced to destination cluster", func(ctx SpecContext) {
				By("Applying Secret to source cluster")
				kubectlApply(exampleFile, sourceClusterContext)

				By("Waiting for Secret to be synchronized")
				Eventually(getSecret).WithContext(ctx).WithTimeout(syncTimeout).WithPolling(pollInterval).WithArguments(
					destinationClusterClient, secretName, secretNamespace).Should(
					And(
						BeEqualToSourceSecret(),
						WithTransform(getSecretMeta, HaveKeessTrackingAnnotations(secretNamespace)),
					),
					fmt.Sprintf("Secret %s/%s should exist within %v and match source secret", secretNamespace, secretName, syncTimeout))
			}, SpecTimeout(mediumT))
		})

		When("the secret is updated on source cluster", func() {
			It("it should be updated in destination cluster", func(ctx SpecContext) {

				By("Applying Secret to source cluster")
				kubectlApply(exampleFile, sourceClusterContext)

				By("Waiting for Secret to be synchronized")
				Eventually(getSecret).WithContext(ctx).WithTimeout(syncTimeout).WithPolling(pollInterval).WithArguments(
					destinationClusterClient, secretName, secretNamespace).Should(Not(BeNil()),
					fmt.Sprintf("Secret %s/%s should exist within %v and match source secret", secretNamespace, secretName, syncTimeout))

				By("Updating Secret in source cluster")
				sourceSecret, err := getSecret(ctx, sourceClusterClient, secretName, secretNamespace)
				Expect(err).NotTo(HaveOccurred())

				// Update existing key and add a new one
				newdata1 := []byte("bmV3cGFzc3dvcmQ=") // "newpassword" in base64
				newdata2 := []byte("bmV3c2VjcmV0")     // "newsecret" in base64
				sourceSecret.Data["database.password"] = newdata1
				sourceSecret.Data["new.secret"] = newdata2

				_, err = sourceClusterClient.CoreV1().Secrets(secretNamespace).Update(ctx, sourceSecret, metav1.UpdateOptions{})
				Expect(err).NotTo(HaveOccurred())

				By("Waiting for updated Secret to be synchronized to destination cluster")
				getSecretData := func(secret *corev1.Secret) map[string][]byte {
					return secret.Data
				}
				Eventually(getSecret).WithContext(ctx).WithTimeout(syncTimeout).WithPolling(pollInterval).WithArguments(
					destinationClusterClient, secretName, secretNamespace).Should(
					And(
						BeEqualToSourceSecret(),
						WithTransform(getSecretMeta, HaveKeessTrackingAnnotations(secretNamespace)),
						WithTransform(getSecretData, HaveKeyWithValue("database.password", newdata1)),
						WithTransform(getSecretData, HaveKeyWithValue("new.secret", newdata2)),
					), fmt.Sprintf("Secret %s/%s should be updated within %v", secretNamespace, secretName, syncTimeout))
			}, SpecTimeout(mediumT))
		})

		When("the secret is deleted from source cluster", Label("secret-delete"), func() {

			BeforeEach(func(ctx SpecContext) {
				By("Ensuring clean start by recreating namespaces on all clusters")
				deleteNamespaceOnAll(ctx, secretNamespace, true)
				createNamespaceOnAll(ctx, secretNamespace)

				By("Applying Secret to source cluster")
				kubectlApply(exampleFile, sourceClusterContext)
			}, NodeTimeout(shortT))

			AfterEach(func(ctx SpecContext) {
				By("Cleaning up by removing test namespace on all clusters")
				deleteNamespaceOnAll(ctx, secretNamespace, false)
			}, NodeTimeout(shortT))

			It("it should delete the orphaned secret from destination cluster", func(ctx SpecContext) {
				By("Waiting for Secret to be synchronized")
				Eventually(getSecret).WithContext(ctx).WithTimeout(syncTimeout).WithPolling(pollInterval).WithArguments(
					destinationClusterClient, secretName, secretNamespace).Should(Not(BeNil()),
					fmt.Sprintf("Secret %s/%s not exists within %v", secretNamespace, secretName, syncTimeout))

				By("Deleting Secret from source cluster")
				err := sourceClusterClient.CoreV1().Secrets(secretNamespace).Delete(ctx, secretName, metav1.DeleteOptions{})
				Expect(err).NotTo(HaveOccurred())

				By("Waiting for orphaned Secret to be deleted from destination cluster")
				Eventually(secretIsNotFound).WithContext(ctx).WithTimeout(syncTimeout).WithPolling(pollInterval).WithArguments(
					destinationClusterClient, secretName, secretNamespace).Should(BeTrue(),
					fmt.Sprintf("Orphaned Secret %s/%s should be deleted within %v", secretNamespace, secretName, syncTimeout))
			}, SpecTimeout(mediumT))
		})
	})
})

// getSecretMeta gets metadata from Secret object.
func getSecretMeta(secret *corev1.Secret) *metav1.ObjectMeta {
	return &secret.ObjectMeta
}

// BeEqualToSourceSecret is a custom matcher to check if a Secret on destination cluster matches the source Secret.
func BeEqualToSourceSecret() types.GomegaMatcher {
	return WithTransform(func(secret *corev1.Secret) bool {
		sourceSecret, err := sourceClusterClient.CoreV1().Secrets(secret.Namespace).Get(context.Background(), secret.Name, metav1.GetOptions{})
		if err != nil {
			return false
		}

		// Secret Sync actually DOES NOT sync labels and annotations.
		// TODO: at some point we should fix that

		// // Check that all labels from source are present in the destination Secret
		// for key, value := range sourceSecret.Labels {
		// 	Expect(secret.Labels).To(HaveKeyWithValue(key, value), fmt.Sprintf("Label %s should match source Secret", key))
		// }

		// // Check that all annotations from source are present in the destination Secret
		// for key, value := range sourceSecret.Annotations {
		// 	Expect(secret.Annotations).To(HaveKeyWithValue(key, value), fmt.Sprintf("Annotation %s should match source Secret", key))
		// }

		// Compare only the Data field, ignoring metadata differences
		return reflect.DeepEqual(secret.Data, sourceSecret.Data)
	}, BeTrue())
}
