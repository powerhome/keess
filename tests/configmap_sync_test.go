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
	configMapExampleFile = filepath.Join("..", "examples", "configmap-sync-example.yaml")
	// configMapName and configMapNamespace must match the example file
	configMapName      = "app-config"
	configMapNamespace = "test-keess"
)

func getConfigMap(client kubernetes.Interface, name, namespace string) (*corev1.ConfigMap, error) {
	return client.CoreV1().ConfigMaps(namespace).Get(context.TODO(), name, metav1.GetOptions{})
}

var _ = Describe("ConfigMap Sync", func() {
	Context("On Cluster mode", func() {

		BeforeEach(func() {
			By("Ensuring clean start by recreating test namespace")
			deleteNamespaceOnAll(configMapNamespace, true)
			createNamespaceOnAll(configMapNamespace)
		})

		AfterEach(func() {
			By("Cleaning up by removing test namespace")
			deleteNamespaceOnAll(configMapNamespace, false)
		})

		When("an annotated ConfigMap is created in the source cluster", func() {

			It("it should be synced to destination cluster", func() {
				By("Applying ConfigMap to source cluster")
				kubectlApply(configMapExampleFile, sourceClusterContext)

				By("Waiting for ConfigMap to be synchronized")
				Eventually(getConfigMap, syncTimeout, pollInterval).WithArguments(
					destinationClusterClient, configMapName, configMapNamespace).Should(
					BeEqualToSourceConfigMap(), fmt.Sprintf("ConfigMap %s/%s should exist within %v and match source configmap", configMapNamespace, configMapName, syncTimeout))
			})
		})

		When("the configmap is updated on source cluster", func() {
			It("it should be updated in destination cluster", func() {

				By("Applying ConfigMap to source cluster")
				kubectlApply(configMapExampleFile, sourceClusterContext)

				By("Waiting for ConfigMap to be synchronized")
				Eventually(getConfigMap, syncTimeout, pollInterval).WithArguments(
					destinationClusterClient, configMapName, configMapNamespace).Should(
					BeEqualToSourceConfigMap(), fmt.Sprintf("ConfigMap %s/%s should exist within %v and match source configmap", configMapNamespace, configMapName, syncTimeout))

				By("Updating ConfigMap in source cluster")
				// we know there is no error because of the previous Eventually check
				sourceConfigMap, _ := getConfigMap(sourceClusterClient, configMapName, configMapNamespace)

				// Update existing key and add a new one
				sourceConfigMap.Data["logging.level"] = "DEBUG"
				sourceConfigMap.Data["new.key"] = "new.value"

				_, err := sourceClusterClient.CoreV1().ConfigMaps(configMapNamespace).Update(context.TODO(), sourceConfigMap, metav1.UpdateOptions{})
				Expect(err).NotTo(HaveOccurred())

				By("Waiting for updated ConfigMap to be synchronized to destination cluster")
				getConfigMapData := func(configMap *corev1.ConfigMap) map[string]string {
					return configMap.Data
				}
				Eventually(getConfigMap, syncTimeout, pollInterval).WithArguments(
					destinationClusterClient, configMapName, configMapNamespace).Should(
					And(
						BeEqualToSourceConfigMap(),
						WithTransform(getConfigMapData, HaveKeyWithValue("logging.level", "DEBUG")),
						WithTransform(getConfigMapData, HaveKeyWithValue("new.key", "new.value")),
					), fmt.Sprintf("ConfigMap %s/%s should be updated within %v", configMapNamespace, configMapName, syncTimeout))
			})
		})

		// TODO: When("the configmap is deleted from source cluster (orphaned)", func() {})
	})
	//TODO: Context("On Namespace mode", func() {})
})
