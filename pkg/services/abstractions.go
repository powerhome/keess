package services

import (
	"strings"
)

// Label that must be applied to the Secrets and ConfigMaps that are managed by keess.
// const ManagedLabelSelector string = "keess.powerhrg.com/managed"
const ManagedLabelSelector string = "testing.keess.powerhrg.com/managed"

// Label that must be applied to the Secrets and ConfigMaps that will be synchronized.
// const LabelSelector string = "keess.powerhrg.com/sync"
const LabelSelector string = "testing.keess.powerhrg.com/sync"

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

// Constant with the annotation created by the kubectl apply command
const KubectlApplyAnnotation string = "kubectl.kubernetes.io/last-applied-configuration"

func splitAndTrim(input string, separator string) []string {
	words := strings.Split(input, separator)
	for i, word := range words {
		words[i] = strings.TrimSpace(word)
	}
	return words
}
