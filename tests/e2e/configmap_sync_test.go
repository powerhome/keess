package e2e_test

import (
	"context"
	"fmt"
	"path/filepath"
	"reflect"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

var (
	configMapExampleFile = filepath.Join("..", "..", "examples", "test-configmap-sync-example.yaml")
	// configMapName and configMapNamespace must match the example file
	configMapName      = "app-config"
	configMapNamespace = "test-keess"
)

// getConfigMap gets a ConfigMap using kubernetes client.
func getConfigMap(ctx context.Context, client kubernetes.Interface, name, namespace string) (*corev1.ConfigMap, error) {
	return client.CoreV1().ConfigMaps(namespace).Get(ctx, name, metav1.GetOptions{})
}

// configMapIsNotFound checks if ConfigMap is not found (shortcut).
func configMapIsNotFound(ctx context.Context, client kubernetes.Interface, name, namespace string) bool {
	_, err := client.CoreV1().ConfigMaps(namespace).Get(ctx, name, metav1.GetOptions{})
	return errors.IsNotFound(err)
}

var _ = Describe("ConfigMap Sync", Label("configmap"), func() {
	Context("On Cluster mode", func() {

		BeforeEach(func(ctx SpecContext) {
			By("Ensuring clean start by recreating test namespace")
			deleteNamespaceOnAll(ctx, configMapNamespace, true)
			createNamespaceOnAll(ctx, configMapNamespace)
		}, NodeTimeout(shortT))

		AfterEach(func(ctx SpecContext) {
			By("Cleaning up by removing test namespace")
			deleteNamespaceOnAll(ctx, configMapNamespace, false)
		}, NodeTimeout(shortT))

		When("an annotated ConfigMap is created in the source cluster", func() {

			It("it should be synced to destination cluster", func(ctx SpecContext) {
				By("Applying ConfigMap to source cluster")
				kubectlApply(configMapExampleFile, sourceClusterContext)

				By("Waiting for ConfigMap to be synchronized")
				Eventually(getConfigMap).WithContext(ctx).WithTimeout(syncTimeout).WithPolling(pollInterval).WithArguments(
					destinationClusterClient, configMapName, configMapNamespace).Should(
					And(
						BeEqualToSourceConfigMap(),
						WithTransform(getConfigMapMeta, HaveKeessTrackingAnnotations(configMapNamespace)),
					), fmt.Sprintf("ConfigMap %s/%s should exist within %v and match source configmap", configMapNamespace, configMapName, syncTimeout))
			}, SpecTimeout(mediumT))
		})

		When("the configmap is updated on source cluster", func() {
			It("it should be updated in destination cluster", func(ctx SpecContext) {

				By("Applying ConfigMap to source cluster")
				kubectlApply(configMapExampleFile, sourceClusterContext)

				By("Waiting for ConfigMap to be synchronized")
				Eventually(getConfigMap).WithContext(ctx).WithTimeout(syncTimeout).WithPolling(pollInterval).WithArguments(
					destinationClusterClient, configMapName, configMapNamespace).Should(Not(BeNil()),
					fmt.Sprintf("ConfigMap %s/%s should exist within %v and match source configmap", configMapNamespace, configMapName, syncTimeout))

				By("Updating ConfigMap in source cluster")
				sourceConfigMap, err := getConfigMap(ctx, sourceClusterClient, configMapName, configMapNamespace)
				Expect(err).NotTo(HaveOccurred())

				// Update existing key and add a new one
				sourceConfigMap.Data["logging.level"] = "DEBUG"
				sourceConfigMap.Data["new.key"] = "new.value"

				_, err = sourceClusterClient.CoreV1().ConfigMaps(configMapNamespace).Update(ctx, sourceConfigMap, metav1.UpdateOptions{})
				Expect(err).NotTo(HaveOccurred())

				By("Waiting for updated ConfigMap to be synchronized to destination cluster")
				getConfigMapData := func(configMap *corev1.ConfigMap) map[string]string {
					return configMap.Data
				}
				Eventually(getConfigMap).WithContext(ctx).WithTimeout(syncTimeout).WithPolling(pollInterval).WithArguments(
					destinationClusterClient, configMapName, configMapNamespace).Should(
					And(
						BeEqualToSourceConfigMap(),
						WithTransform(getConfigMapMeta, HaveKeessTrackingAnnotations(configMapNamespace)),
						WithTransform(getConfigMapData, HaveKeyWithValue("logging.level", "DEBUG")),
						WithTransform(getConfigMapData, HaveKeyWithValue("new.key", "new.value")),
					), fmt.Sprintf("ConfigMap %s/%s should be updated within %v", configMapNamespace, configMapName, syncTimeout))
			}, SpecTimeout(mediumT))
		})

		When("the configmap is deleted from source cluster", Label("cm-delete"), func() {

			BeforeEach(func(ctx SpecContext) {
				By("Ensuring clean start by recreating namespaces on all clusters")
				deleteNamespaceOnAll(ctx, configMapNamespace, true)
				createNamespaceOnAll(ctx, configMapNamespace)

				By("Applying ConfigMap to source cluster")
				kubectlApply(configMapExampleFile, sourceClusterContext)
			}, NodeTimeout(shortT))

			AfterEach(func(ctx SpecContext) {
				By("Cleaning up by removing test namespace on all clusters")
				deleteNamespaceOnAll(ctx, configMapNamespace, false)
			}, NodeTimeout(shortT))

			It("it should delete the orphaned configmap from destination cluster", func(ctx SpecContext) {
				By("Waiting for ConfigMap to be synchronized")
				Eventually(getConfigMap).WithContext(ctx).WithTimeout(syncTimeout).WithPolling(pollInterval).WithArguments(
					destinationClusterClient, configMapName, configMapNamespace).Should(Not(BeNil()),
					fmt.Sprintf("ConfigMap %s/%s not exists within %v", configMapNamespace, configMapName, syncTimeout))

				By("Deleting ConfigMap from source cluster")
				err := sourceClusterClient.CoreV1().ConfigMaps(configMapNamespace).Delete(ctx, configMapName, metav1.DeleteOptions{})
				Expect(err).NotTo(HaveOccurred())

				By("Waiting for orphaned ConfigMap to be deleted from destination cluster")
				Eventually(configMapIsNotFound).WithContext(ctx).WithTimeout(syncTimeout).WithPolling(pollInterval).WithArguments(
					destinationClusterClient, configMapName, configMapNamespace).Should(BeTrue(),
					fmt.Sprintf("Orphaned ConfigMap %s/%s should be deleted within %v", configMapNamespace, configMapName, syncTimeout))
			}, SpecTimeout(mediumT))
		})
	})
	//TODO: Context("On Namespace mode", func() {})
})

// getConfigMapMeta gets metadata from ConfigMap object.
func getConfigMapMeta(config *corev1.ConfigMap) *metav1.ObjectMeta {
	return &config.ObjectMeta
}

// BeEqualToSourceConfigMap is a custom matcher to check if a ConfigMap on destination cluster matches the source ConfigMap.
func BeEqualToSourceConfigMap() types.GomegaMatcher {
	return WithTransform(func(configmap *corev1.ConfigMap) bool {
		sourceConfigMap, err := sourceClusterClient.CoreV1().ConfigMaps(configmap.Namespace).Get(context.Background(), configmap.Name, metav1.GetOptions{})
		if err != nil {
			return false
		}

		// Compare only the Data field, ignoring metadata differences
		return reflect.DeepEqual(configmap.Data, sourceConfigMap.Data)
	}, BeTrue())
}
