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
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
)

var (
	serviceExampleFile = filepath.Join("..", "..", "examples", "test-service-sync-example.yaml")
	// serviceName, serviceNamespace and servicePort must match the example file
	serviceName                      = "mysql-svc"
	serviceNamespace                 = "test-keess-service"
	serviceExampleConflictingFile    = filepath.Join("..", "..", "examples", "wrong", "test-service-sync-example-conflict.yaml")
	serviceExampleLocalEndpointsFile = filepath.Join("..", "..", "examples", "wrong", "test-service-sync-example-local-endpoints.yaml")
	// podName must match the example file for local endpoints
	podName     = "mysql-other-pod"
	servicePort = 3306
)

// getService gets a Service using kubernetes client (shortcut).
func getService(client kubernetes.Interface, name, namespace string) (*corev1.Service, error) {
	return client.CoreV1().Services(namespace).Get(context.TODO(), name, metav1.GetOptions{})
}

// serviceIsNotFound checks if Service is not found (shortcut).
func serviceIsNotFound(client kubernetes.Interface, name, namespace string) bool {
	_, err := client.CoreV1().Services(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	return errors.IsNotFound(err)
}

// getServicePorts gets Service Ports (shortcut).
func getServicePorts(service *corev1.Service) []corev1.ServicePort {
	return service.Spec.Ports
}

// getPodReadiness returns if a pod exists and is in ready state.
func getPodReadiness(client kubernetes.Interface, podName, namespace string) (bool, error) {
	pod, err := client.CoreV1().Pods(namespace).Get(context.TODO(), podName, metav1.GetOptions{})
	if err != nil {
		return false, err
	}

	// Check if pod is ready by examining the PodReady condition
	for _, condition := range pod.Status.Conditions {
		if condition.Type == corev1.PodReady {
			return condition.Status == corev1.ConditionTrue, nil
		}
	}
	return false, nil
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
			// createNamespaceOnAll(serviceNamespace)
			createNamespace(sourceClusterClient, serviceNamespace)

			By("Applying Service to source cluster")
			kubectlApply(serviceExampleFile, sourceClusterContext)
		})

		AfterEach(func() {
			By("Cleaning up by removing test namespace on all clusters")
			deleteNamespaceOnAll(serviceNamespace, false)
		})

		It("it should NOT delete the service if it has local endpoints", func() {
			By("Waiting for Service to be synchronized")
			Eventually(getService, syncTimeout, pollInterval).WithArguments(
				destinationClusterClient, serviceName, serviceNamespace).Should(Not(BeNil()),
				fmt.Sprintf("Service %s/%s not exists within %v", serviceNamespace, serviceName, syncTimeout))

			By("Changing the service on destination cluster, adding local endpoints")
			kubectlApply(serviceExampleLocalEndpointsFile, destinationClusterContext)

			By("Waiting for pod on destination cluster to be ready")
			Eventually(getPodReadiness, syncTimeout, pollInterval).WithArguments(
				destinationClusterClient, podName, serviceNamespace).Should(BeTrue(),
				fmt.Sprintf("Pod %s/%s not ready within %v", serviceNamespace, podName, syncTimeout))

			By("Deleting Service from source cluster")
			err := sourceClusterClient.CoreV1().Services(serviceNamespace).Delete(context.TODO(), serviceName, metav1.DeleteOptions{})
			Expect(err).NotTo(HaveOccurred())

			By("Checking that orphaned Service is not deleted from dest cluster")
			Consistently(serviceIsNotFound, syncTimeout, pollInterval).WithArguments(
				destinationClusterClient, serviceName, serviceNamespace).Should(BeFalse(),
				fmt.Sprintf("Orphaned Service %s/%s should not be deleted (tested for %v)", serviceNamespace, serviceName, syncTimeout))
		})

		It("it should delete the orphaned service from destination cluster", func() {
			By("Waiting for Service to be synchronized")
			Eventually(getService, syncTimeout, pollInterval).WithArguments(
				destinationClusterClient, serviceName, serviceNamespace).Should(Not(BeNil()),
				fmt.Sprintf("Service %s/%s not exists within %v", serviceNamespace, serviceName, syncTimeout))

			By("Deleting Service from source cluster")
			err := sourceClusterClient.CoreV1().Services(serviceNamespace).Delete(context.TODO(), serviceName, metav1.DeleteOptions{})
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for orphaned Service to be deleted from destination cluster")
			Eventually(serviceIsNotFound, syncTimeout, pollInterval).WithArguments(
				destinationClusterClient, serviceName, serviceNamespace).Should(BeTrue(),
				fmt.Sprintf("Orphaned Service %s/%s should be deleted within %v", serviceNamespace, serviceName, syncTimeout))
		})

	})
})

// HaveCiliumAnnotations is a custom matcher to check for Cilium annotations.
func HaveCiliumAnnotations() types.GomegaMatcher {
	return WithTransform(func(metadata *metav1.ObjectMeta) bool {
		Expect(metadata.Annotations).To(HaveKeyWithValue("service.cilium.io/global", "true"), "Destination Service should have Cilium global annotation")
		Expect(metadata.Annotations).To(HaveKeyWithValue("service.cilium.io/shared", "false"), "Destination Service should have Cilium shared annotation set to false")
		return true
	}, BeTrue())
}

// HaveEmptySelector is a Gomega custom matcher for checking if Service has an empty Selector.
func HaveEmptySelector() types.GomegaMatcher {
	return WithTransform(func(service *corev1.Service) bool {
		Expect(service.Spec.Selector).To(BeEmpty(), "Destination Service should have empty selector")
		return true
	}, BeTrue())
}
