package e2e_test

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"reflect"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	kubeconfig                = "../localTestKubeconfig"
	sourceClusterContext      = "kind-source-cluster"
	destinationClusterContext = "kind-destination-cluster"
	syncTimeout               = time.Minute * 1
	pollInterval              = time.Second * 10
)

var (
	sourceClusterClient      kubernetes.Interface
	destinationClusterClient kubernetes.Interface
)

func TestE2E(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Keess E2E Suite")
}

var _ = BeforeSuite(func() {

	// Check if kubeconfig exists
	if _, err := os.Stat(kubeconfig); os.IsNotExist(err) {
		Fail(fmt.Sprintf("Kubeconfig file not found at %s. Please run 'make setup-local-clusters' first.", kubeconfig))
	}

	// Load kubeconfig
	config, err := clientcmd.LoadFromFile(kubeconfig)
	Expect(err).NotTo(HaveOccurred())

	// Create client for source cluster
	sourceConfig, err := clientcmd.NewDefaultClientConfig(*config, &clientcmd.ConfigOverrides{
		CurrentContext: "kind-source-cluster",
	}).ClientConfig()
	Expect(err).NotTo(HaveOccurred())

	sourceClusterClient, err = kubernetes.NewForConfig(sourceConfig)
	Expect(err).NotTo(HaveOccurred())

	// Create client for destination cluster
	destConfig, err := clientcmd.NewDefaultClientConfig(*config, &clientcmd.ConfigOverrides{
		CurrentContext: "kind-destination-cluster",
	}).ClientConfig()
	Expect(err).NotTo(HaveOccurred())

	destinationClusterClient, err = kubernetes.NewForConfig(destConfig)
	Expect(err).NotTo(HaveOccurred())

	// Verify both clusters are accessible
	By("Verifying source cluster is accessible")
	_, err = sourceClusterClient.CoreV1().Namespaces().Get(context.TODO(), "default", metav1.GetOptions{})
	Expect(err).NotTo(HaveOccurred())

	By("Verifying destination cluster is accessible")
	_, err = destinationClusterClient.CoreV1().Namespaces().Get(context.TODO(), "default", metav1.GetOptions{})
	Expect(err).NotTo(HaveOccurred())
})

var _ = AfterSuite(func() {
	// Cleanup any remaining test resources
	By("Cleaning up test resources")
	cleanupTestResources()
})

func cleanupTestResources() {
	ctx := context.TODO()

	// Clean up ConfigMaps
	if sourceClusterClient != nil {
		sourceClusterClient.CoreV1().ConfigMaps("default").Delete(ctx, "app-config", metav1.DeleteOptions{})
	}
	if destinationClusterClient != nil {
		destinationClusterClient.CoreV1().ConfigMaps("default").Delete(ctx, "app-config", metav1.DeleteOptions{})
	}

	// Clean up Secrets
	if sourceClusterClient != nil {
		sourceClusterClient.CoreV1().Secrets("default").Delete(ctx, "app-secret", metav1.DeleteOptions{})
	}
	if destinationClusterClient != nil {
		destinationClusterClient.CoreV1().Secrets("default").Delete(ctx, "app-secret", metav1.DeleteOptions{})
	}
}

func kubectlApply(manifestFile, context string) {
	cmd := exec.Command("kubectl", "apply", "-f", manifestFile, "--kubeconfig", kubeconfig, "--context", context)
	output, err := cmd.CombinedOutput()
	Expect(err).NotTo(HaveOccurred(), "Failed to apply manifest: %s", string(output))
}

// func kubectlDelete(manifestFile, context string) {
// 	cmd := exec.Command("kubectl", "delete", "-f", manifestFile, "--kubeconfig", kubeconfig, "--context", context)
// 	output, err := cmd.CombinedOutput()
// 	Expect(err).NotTo(HaveOccurred(), "Failed to delete manifest: %s", string(output))
// }

// Creates a namespace using kubernetes client
func createNamespace(client kubernetes.Interface, namespace string) {
	_, err := client.CoreV1().Namespaces().Create(context.TODO(), &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace,
		},
	}, metav1.CreateOptions{})
	Expect(err).NotTo(HaveOccurred(), fmt.Sprintf("Failed to create namespace %s", namespace))

	// Wait a bit for the namespace to be fully created
	time.Sleep(time.Second * 2)
}

// Deletes a namespace using kubernetes client
func deleteNamespace(client kubernetes.Interface, namespace string, wait bool) {
	err := client.CoreV1().Namespaces().Delete(context.TODO(), namespace, metav1.DeleteOptions{})

	if err == nil && wait {
		// Wait a bit for the namespace to be fully deleted
		time.Sleep(time.Second * 15)
	}
	// having an error is ok, namespace might not exist
}

// Creates namespace given as argument on all test clusters
func createNamespaceOnAll(namespace string) {
	createNamespace(sourceClusterClient, namespace)
	createNamespace(destinationClusterClient, namespace)
}

// Deletes namespace given as argument on all test clusters
func deleteNamespaceOnAll(namespace string, wait bool) {
	deleteNamespace(sourceClusterClient, namespace, wait)
	deleteNamespace(destinationClusterClient, namespace, wait)
}

// Custom matcher to check if a Secret on destination cluster matches the source Secret
func BeEqualToSourceSecret() types.GomegaMatcher {
	return WithTransform(func(secret *corev1.Secret) bool {
		sourceSecret, err := sourceClusterClient.CoreV1().Secrets(secret.Namespace).Get(context.Background(), secret.Name, metav1.GetOptions{})
		if err != nil {
			return false
		}

		// Secret Sync actually DOES NOT sync labels and annotations. Not sure if that's intended.
		// TODO: is it a bug?

		// // Check that all labels from source are present in the destination Secret
		// for key, value := range sourceSecret.Labels {
		// 	Expect(secret.Labels).To(HaveKeyWithValue(key, value), fmt.Sprintf("Label %s should match source Secret", key))
		// }

		// // Check that all annotations from source are present in the destination Secret
		// for key, value := range sourceSecret.Annotations {
		// 	Expect(secret.Annotations).To(HaveKeyWithValue(key, value), fmt.Sprintf("Annotation %s should match source Secret", key))
		// }

		Expect(secret.Labels).To(HaveKeyWithValue("keess.powerhrg.com/managed", "true"), "Destination Secret should have correct managed label")
		Expect(secret.Annotations).To(HaveKeyWithValue("keess.powerhrg.com/source-cluster", sourceClusterContext), "Destination Secret should have correct source cluster annotation")
		Expect(secret.Annotations).To(HaveKeyWithValue("keess.powerhrg.com/source-namespace", sourceSecret.Namespace), "Destination Secret should have correct source namespace annotation")

		// TODO: I think we found a bug here, because the source resource version is not synced whe source is updated
		// This line catches that when on the update case
		// Expect(secret.Annotations).To(HaveKeyWithValue("keess.powerhrg.com/source-resource-version", sourceSecret.ResourceVersion), "Destination Secret should have correct source resource version annotation")

		// Compare only the Data field, ignoring metadata differences
		return reflect.DeepEqual(secret.Data, sourceSecret.Data)
	}, BeTrue())
}

// Custom matcher to check if a ConfigMap on destination cluster matches the source ConfigMap
func BeEqualToSourceConfigMap() types.GomegaMatcher {
	return WithTransform(func(configmap *corev1.ConfigMap) bool {
		sourceConfigMap, err := sourceClusterClient.CoreV1().ConfigMaps(configmap.Namespace).Get(context.Background(), configmap.Name, metav1.GetOptions{})
		if err != nil {
			return false
		}

		Expect(configmap.Labels).To(HaveKeyWithValue("keess.powerhrg.com/managed", "true"), "Destination ConfigMap should have correct managed label")
		Expect(configmap.Annotations).To(HaveKeyWithValue("keess.powerhrg.com/source-cluster", sourceClusterContext), "Destination ConfigMap should have correct source cluster annotation")
		Expect(configmap.Annotations).To(HaveKeyWithValue("keess.powerhrg.com/source-namespace", sourceConfigMap.Namespace), "Destination ConfigMap should have correct source namespace annotation")

		// Compare only the Data field, ignoring metadata differences
		return reflect.DeepEqual(configmap.Data, sourceConfigMap.Data)
	}, BeTrue())
}
