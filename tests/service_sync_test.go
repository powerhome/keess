package e2e_test

import (
	"context"
	"fmt"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"github.com/onsi/gomega/types"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
)

var (
	serviceExampleFile = filepath.Join("..", "examples", "test-service-sync-example.yaml")
	// serviceName, serviceNamespace and servicePort must match the example file
	serviceName                   = "mysql-svc"
	serviceNamespace              = "test-keess-service"
	serviceExampleConflictingFile = filepath.Join("..", "examples", "test-service-sync-example-conflict.yaml")
	servicePort                   = 3306
)

// Get Service shortcut
func getService(client kubernetes.Interface, name, namespace string) (*corev1.Service, error) {
	return client.CoreV1().Services(namespace).Get(context.TODO(), name, metav1.GetOptions{})
}

// Get Namespace shortcut
func getNamespace(client kubernetes.Interface, name string) (*corev1.Namespace, error) {
	return client.CoreV1().Namespaces().Get(context.TODO(), name, metav1.GetOptions{})
}

// Get Service Ports shortcut
func getServicePorts(service *corev1.Service) []corev1.ServicePort {
	return service.Spec.Ports
}

var _ = Describe("Service Cluster Sync", Label("service"), func() {

	// Note this is an ordered test container, so each "It" will build upon the previous one
	When("an annotated Service is created in the source cluster", Ordered, func() {
		BeforeAll(func() {
			By("Ensuring clean start by removing namespaces on all clusters")
			deleteNamespaceOnAll(serviceNamespace, true)

			By("Creating namespace only on source, to force keess to create it on destination")
			createNamespace(sourceClusterClient, serviceNamespace)

			By("Applying Service to source cluster")
			kubectlApply(serviceExampleFile, sourceClusterContext)

		})

		AfterAll(func() {
			By("Cleaning up by removing test namespace on all clusters")
			deleteNamespaceOnAll(serviceNamespace, false)
		})

		It("it should create namespace on destination if it does not exist", func() {
			Eventually(getNamespace, syncTimeout, pollInterval).WithArguments(
				destinationClusterClient, serviceNamespace).Should(
				And(
					Not(BeNil()),
					WithTransform(getMetadata, HaveKeessTrackingAnnotations(serviceNamespace)),
				), fmt.Sprintf("Namespace %s not created on destination cluster within %v", serviceNamespace, syncTimeout))
		})

		It("it should be synced to destination cluster", func() {
			// Note: we do NOT test for the destination Service Endpoints here, or for its reachability
			// We test for Cilium's annotations and assume Cilium will do its job providing the connectivity
			// Reasons:
			// - It would be kind of testing Cilium's functionality from here, but maybe that would be ok because this is a n E2E test
			// - Cilium does not update the Service's Endpoints on destination Service, unless an additional beta flag is set:
			//   https://docs.cilium.io/en/latest/network/clustermesh/services/#synchronizing-kubernetes-endpointslice-beta
			Eventually(getService, syncTimeout, pollInterval).WithArguments(
				destinationClusterClient, serviceName, serviceNamespace).Should(
				And(
					Not(BeNil()),
					WithTransform(getMetadata, HaveKeessTrackingAnnotations(serviceNamespace)),
					WithTransform(getMetadata, HaveCiliumAnnotations()),
					HaveEmptySelector(),
				), fmt.Sprintf("Service %s/%s not synchronized as expected within %v", serviceNamespace, serviceName, syncTimeout))
		})

		It("it should sync Service updates in destination cluster", func() {
			By("Updating Service in source cluster by adding a port")
			// we know there is no error because of the previous Eventually check
			sourceService, _ := getService(sourceClusterClient, serviceName, serviceNamespace)

			// Add a new port
			sourceService.Spec.Ports = append(sourceService.Spec.Ports, corev1.ServicePort{
				Name:       "http",
				Port:       8080,
				Protocol:   corev1.ProtocolTCP,
				TargetPort: intstr.FromInt(8080),
			})

			updatedService, err := sourceClusterClient.CoreV1().Services(serviceNamespace).Update(context.TODO(), sourceService, metav1.UpdateOptions{})
			Expect(err).NotTo(HaveOccurred())
			sourceRevision := updatedService.ResourceVersion

			By("Waiting and verifying the port is added to the destination cluster Service")
			Eventually(getService, syncTimeout, pollInterval).WithArguments(
				destinationClusterClient, serviceName, serviceNamespace).Should(
				And(
					WithTransform(getMetadata, HaveRevisionMatchingSource(sourceRevision)),
					WithTransform(getServicePorts, HaveLen(2)),
					WithTransform(getServicePorts, ContainElement(MatchFields(IgnoreExtras, Fields{
						"Name": Equal("http"),
						"Port": Equal(int32(8080)),
					}))),
				), fmt.Sprintf("Service %s/%s should be updated within %v with the new port", serviceNamespace, serviceName, syncTimeout))
		})

	})

	When("the service already exists on destination cluster", Label("service-conflict"), func() {
		BeforeEach(func() {
			By("Ensuring clean start by recreating namespaces on all clusters")
			deleteNamespaceOnAll(serviceNamespace, true)
			createNamespaceOnAll(serviceNamespace)

			By("Create service with same name on destination cluster")
			kubectlApply(serviceExampleConflictingFile, destinationClusterContext)

			By("Applying Service to source cluster")
			kubectlApply(serviceExampleFile, sourceClusterContext)
		})

		AfterEach(func() {
			By("Cleaning up by removing test namespace on all clusters")
			deleteNamespaceOnAll(serviceNamespace, false)
		})

		It("it should not sync if Service exists on destination", func() {
			By("Verifying the destination service is NOT updated with the source service port")
			Consistently(getService, syncTimeout, pollInterval).WithArguments(
				destinationClusterClient, serviceName, serviceNamespace).Should(
				And(
					WithTransform(getServicePorts, HaveLen(1)),
					WithTransform(getServicePorts, Not(ContainElement(MatchFields(IgnoreExtras, Fields{
						"Port": Equal(int32(servicePort)),
					})))),
				), fmt.Sprintf("Service %s/%s on destination should NOT be updated with the port from source-cluster service", serviceNamespace, serviceName))
		})
	})

	When("the service is deleted from source cluster", Label("service-delete"), func() {

		BeforeEach(func() {
			By("Ensuring clean start by recreating namespaces on all clusters")
			deleteNamespaceOnAll(serviceNamespace, true)
			createNamespaceOnAll(serviceNamespace)

			By("Applying Service to source cluster")
			kubectlApply(serviceExampleFile, sourceClusterContext)
		})

		// It("it should NOT delete the service if it has local endpoints", func() {})

		It("it should delete the orphaned service from destination cluster", func() {
			By("Waiting for Service to be synchronized")
			Eventually(getService, syncTimeout, pollInterval).WithArguments(
				destinationClusterClient, serviceName, serviceNamespace).Should(Not(BeNil()),
				fmt.Sprintf("Service %s/%s not exists within %v", serviceNamespace, serviceName, syncTimeout))

			By("Deleting Service from source cluster")
			err := sourceClusterClient.CoreV1().Services(serviceNamespace).Delete(context.TODO(), serviceName, metav1.DeleteOptions{})
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for orphaned Service to be deleted from destination cluster")
			Eventually(getService, syncTimeout, pollInterval).WithArguments(
				destinationClusterClient, serviceName, serviceNamespace).Should(BeNil(),
				fmt.Sprintf("Orphaned Service %s/%s should be deleted within %v", serviceNamespace, serviceName, syncTimeout))
		})

		// It("it should NOT delete a non-empty namespace on destination even it's was managed by Keess", func() {})
		// It("it should delete an EMPTY namespace on destination IF it's managed by Keess", func() {})
	})

	// When("a managed service has local endpoints", func() {
	// 	It("should not be deleted even if source is removed", func() {
	// 		By("Creating a service with local endpoints on destination cluster")
	// 		localService := &corev1.Service{
	// 			ObjectMeta: metav1.ObjectMeta{
	// 				Name:      "local-service",
	// 				Namespace: serviceNamespace,
	// 				Labels: map[string]string{
	// 					"keess.powerhrg.com/managed": "true",
	// 				},
	// 				Annotations: map[string]string{
	// 					"keess.powerhrg.com/source-cluster":          "kind-source-cluster",
	// 					"keess.powerhrg.com/source-namespace":        serviceNamespace,
	// 					"keess.powerhrg.com/source-resource-version": "123",
	// 					"service.cilium.io/global":                   "true",
	// 					"service.cilium.io/shared":                   "false",
	// 				},
	// 			},
	// 			Spec: corev1.ServiceSpec{
	// 				Ports: []corev1.ServicePort{
	// 					{
	// 						Name:       "http",
	// 						Port:       80,
	// 						Protocol:   corev1.ProtocolTCP,
	// 						TargetPort: intstr.FromInt(8080),
	// 					},
	// 				},
	// 				Selector: map[string]string{
	// 					"app": "local-app", // Non-empty selector indicates local endpoints
	// 				},
	// 				Type: corev1.ServiceTypeClusterIP,
	// 			},
	// 		}

	// 		_, err := destinationClusterClient.CoreV1().Services(serviceNamespace).Create(context.TODO(), localService, metav1.CreateOptions{})
	// 		Expect(err).NotTo(HaveOccurred())

	// 		By("Verifying service with local endpoints is not deleted during orphan cleanup")
	// 		Consistently(func() error {
	// 			_, err := getService(destinationClusterClient, "local-service", serviceNamespace)
	// 			return err
	// 		}, time.Second*30, time.Second*5).Should(BeNil(), "Service with local endpoints should not be deleted")

	// 		By("Cleaning up the test service")
	// 		err = destinationClusterClient.CoreV1().Services(serviceNamespace).Delete(context.TODO(), "local-service", metav1.DeleteOptions{})
	// 		Expect(err).NotTo(HaveOccurred())
	// 	})
	// })

	// TODO: When("the service is deleted from source cluster (orphaned)", func() {})
})

func HaveCiliumAnnotations() types.GomegaMatcher {
	return WithTransform(func(metadata *metav1.ObjectMeta) bool {
		Expect(metadata.Annotations).To(HaveKeyWithValue("service.cilium.io/global", "true"), "Destination Service should have Cilium global annotation")
		Expect(metadata.Annotations).To(HaveKeyWithValue("service.cilium.io/shared", "false"), "Destination Service should have Cilium shared annotation set to false")
		return true
	}, BeTrue())
}

// Gomega custom matcher for checking if Service has an empty Selector
func HaveEmptySelector() types.GomegaMatcher {
	return WithTransform(func(service *corev1.Service) bool {
		Expect(service.Spec.Selector).To(BeEmpty(), "Destination Service should have empty selector")
		return true
	}, BeTrue())
}
