package e2e_test

import (
	"context"
	"fmt"
	"os"
	"os/exec"
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

// TODO: check if this function makes sense
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

	// Clean up Services
	if sourceClusterClient != nil {
		sourceClusterClient.CoreV1().Services("my-namespace").Delete(ctx, "mysql-svc", metav1.DeleteOptions{})
	}
	if destinationClusterClient != nil {
		destinationClusterClient.CoreV1().Services("my-namespace").Delete(ctx, "mysql-svc", metav1.DeleteOptions{})
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

// Custom matcher to check if metadata has the Keess tracking annotations
// It also checks if the source resource version annotation matches the expected source revision
func HaveKeessTrackingAnnotations(sourceNamespace string) types.GomegaMatcher {
	return WithTransform(func(metadata *metav1.ObjectMeta) bool {
		// NOTE: we can only use Expect here, because the Service is already created with
		// these annotations, so we don't need to wait for they eventually be synchronized.
		// Expect fails the test immediately if the annotations are not present, breaking the Eventually() loop
		Expect(metadata.Labels).To(HaveKeyWithValue("keess.powerhrg.com/managed", "true"), "Destination object should have correct managed label")
		Expect(metadata.Annotations).To(HaveKeyWithValue("keess.powerhrg.com/source-cluster", sourceClusterContext), "Destination object should have correct source cluster annotation")
		Expect(metadata.Annotations).To(HaveKeyWithValue("keess.powerhrg.com/source-namespace", sourceNamespace), "Destination object should have correct source namespace annotation")
		return true
	}, BeTrue())
}

func HaveRevisionMatchingSource(expectedRevision string) types.GomegaMatcher {
	return WithTransform(func(metadata *metav1.ObjectMeta) bool {
		// NOTE: Cannot use Expect here, because it could fail the test immediately. We got to let Eventually keep try this.
		revNote, ok := metadata.Annotations["keess.powerhrg.com/source-resource-version"]
		if ok && revNote == expectedRevision {
			return true
		}
		GinkgoWriter.Printf("Expected source resource version annotation '%s' but got '%s'", expectedRevision, revNote)
		return false
	}, BeTrue())
}

// Generic function to get ObjectMeta from any Kubernetes object
func getMetadata(obj metav1.Object) *metav1.ObjectMeta {
	// Get the actual ObjectMeta from the object
	switch o := obj.(type) {
	case *corev1.Service:
		return &o.ObjectMeta
	case *corev1.Secret:
		return &o.ObjectMeta
	case *corev1.ConfigMap:
		return &o.ObjectMeta
	case *corev1.Namespace:
		return &o.ObjectMeta
	default:
		// For any other object, return empty metadata
		return &metav1.ObjectMeta{}
	}
}
