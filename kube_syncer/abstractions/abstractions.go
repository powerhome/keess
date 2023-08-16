package abstractions

import (
	"strings"
	"time"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// Label that must be applied to the Secrets and ConfigMaps that are managed by keess.
const ManagedLabelSelector string = "keess.powerhrg.com/managed"

// Label that must be applied to the Secrets and ConfigMaps that will be synchronized.
const LabelSelector string = "keess.powerhrg.com/sync"

// Accepted annotation to configure the synchronization across clusters.
const ClusterAnnotation string = "keess.powerhrg.com/clusters"

// Accepted annotation to configure the synchronization across namespaces.
const NamespaceNameAnnotation string = "keess.powerhrg.com/namespaces-names"

// Accepted annotation to configure the synchronization across namespaces.
const NamespaceLabelAnnotation string = "keess.powerhrg.com/namespace-label"

// Annotation with the source cluster of the object managed by kees.
const SourceClusterAnnotation string = "keess.powerhrg.com/source-cluster"

// Annotation with the source namespace of the object managed by kees.
const SourceNamespaceAnnotation string = "keess.powerhrg.com/source-namespace"

// Annotation with the source resource version of the object managed by kees.
const SourceResourceVersionAnnotation string = "keess.powerhrg.com/source-resource-version"

// Constant that represents the synchronization across all namespaces.
const All string = "all"

// Constant with the annotation created by the kubectl apply command
const KubectlApplyAnnotation string = "kubectl.kubernetes.io/last-applied-configuration"

// The timeout for watching.
var WatchTimeOut int64 = int64(time.Duration(60 * 60 * 24 * 365 * 10))

// Logger object.
var Logger *zap.SugaredLogger

// A list with all existent namespaces.
var Namespaces map[string]*corev1.Namespace = map[string]*corev1.Namespace{}

// A map containing the ConfigMaps that sould be present in every Namespace
var EntitiesToAllNamespaces map[string]map[string]runtime.Object = make(map[string]map[string]runtime.Object)

// A map containing the ConfigMaps that sould be present in every Namespace that matches with the configured label
var EntitiesToLabeledNamespaces map[string]map[string]runtime.Object = make(map[string]map[string]runtime.Object)

// === Functions === //

// Check if exists a valid annotation in an annotation map.
func ContainsAValidAnnotation(annotations map[string]string) bool {
	contains := false

	for key := range annotations {
		if key == ClusterAnnotation || key == NamespaceNameAnnotation || key == NamespaceLabelAnnotation {
			contains = true
			break
		}
	}

	return contains
}

// Check if a label value is valid.
func IsAValidLabelValue(labelValue string) bool {
	switch labelValue {
	case string(Namespace):
		return true
	case string(Cluster):
		return true
	default:
		return false
	}
}

// Check if an eventType is valid.
func IsAValidEvent(eventType string) bool {
	switch eventType {
	case string(Added):
		return true
	case string(Modified):
		return true
	case string(Deleted):
		return true
	default:
		return false
	}
}

// Returns a slice with the items in a list like: item1, item2, item3.
func StringToSlice(text string) []string {
	var slices []string

	if !strings.Contains(text, ",") {
		slices = append(slices, strings.Trim(text, " "))
		return slices
	}

	// Getting the namespaces to replicate
	for _, slice := range strings.Split(text, ",") {
		slices = append(slices, strings.Trim(slice, " "))
	}

	return slices
}
