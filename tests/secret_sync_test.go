package e2e_test

import (
	"context"
	"fmt"

	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

var (
	exampleFile = filepath.Join("..", "examples", "secret-sync-example.yaml")
	// secretName and namespace must match the example file
	secretName      = "app-secret"
	secretNamespace = "test-keess"
)

func getSecret(client kubernetes.Interface, name, namespace string) (*corev1.Secret, error) {
	return client.CoreV1().Secrets(namespace).Get(context.TODO(), name, metav1.GetOptions{})
}

var _ = Describe("Secret Sync", func() {
	Context("On Cluster mode", func() {

		BeforeEach(func() {
			By("Ensuring clean start by recreating test namespace")
			deleteNamespaceOnAll(secretNamespace, true)
			createNamespaceOnAll(secretNamespace)
		})

		AfterEach(func() {
			By("Cleaning up by removing test namespace")
			deleteNamespaceOnAll(secretNamespace, false)
		})

		When("an annotated Secret is created in the source cluster", func() {

			It("it should be synced to destination cluster", func() {
				By("Applying Secret to source cluster")
				kubectlApply(exampleFile, sourceClusterContext)

				By("Waiting for Secret to be synchronized")
				Eventually(getSecret, syncTimeout, pollInterval).WithArguments(
					destinationClusterClient, secretName, secretNamespace).Should(
					BeEqualToSourceSecret(), fmt.Sprintf("Secret %s/%s should exist within %v and match source secret", secretNamespace, secretName, syncTimeout))
			})
		})

		When("the secret is updated on source cluster", func() {
			It("it should be updated in destination cluster", func() {

				By("Applying Secret to source cluster")
				kubectlApply(exampleFile, sourceClusterContext)

				By("Waiting for Secret to be synchronized")
				Eventually(getSecret, syncTimeout, pollInterval).WithArguments(
					destinationClusterClient, secretName, secretNamespace).Should(
					BeEqualToSourceSecret(), fmt.Sprintf("Secret %s/%s should exist within %v and match source secret", secretNamespace, secretName, syncTimeout))

				By("Updating Secret in source cluster")
				// we know there is no error because of the previous Eventually check
				sourceSecret, _ := getSecret(sourceClusterClient, secretName, secretNamespace)

				// Update existing key and add a new one
				newdata1 := []byte("bmV3cGFzc3dvcmQ=") // "newpassword" in base64
				newdata2 := []byte("bmV3c2VjcmV0")     // "newsecret" in base64
				sourceSecret.Data["database.password"] = newdata1
				sourceSecret.Data["new.secret"] = newdata2

				_, err := sourceClusterClient.CoreV1().Secrets(secretNamespace).Update(context.TODO(), sourceSecret, metav1.UpdateOptions{})
				Expect(err).NotTo(HaveOccurred())

				By("Waiting for updated Secret to be synchronized to destination cluster")
				getSecretData := func(secret *corev1.Secret) map[string][]byte {
					return secret.Data
				}
				Eventually(getSecret, syncTimeout, pollInterval).WithArguments(
					destinationClusterClient, secretName, secretNamespace).Should(
					And(
						BeEqualToSourceSecret(),
						WithTransform(getSecretData, HaveKeyWithValue("database.password", newdata1)),
						WithTransform(getSecretData, HaveKeyWithValue("new.secret", newdata2)),
					), fmt.Sprintf("Secret %s/%s should be updated within %v", secretNamespace, secretName, syncTimeout))
			})
		})

		// TODO: When("the secret is deleted from source cluster (orphaned)", func() {})
	})
	//TODO: Context("On Namespace mode", func() {})
})
