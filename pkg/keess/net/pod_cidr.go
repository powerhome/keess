package net

import (
	"context"
	"fmt"
	"net"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

// GetPodCIDRs discovers the pod addressing CIDR ranges for the cluster by querying all nodes.
// It collects unique CIDRs from both node.Spec.PodCIDR and node.Spec.PodCIDRs fields.
func GetPodCIDRs(ctx context.Context, coreV1 v1.CoreV1Interface) ([]string, error) {

	nodes, err := coreV1.Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list nodes: %w", err)
	}

	if len(nodes.Items) == 0 {
		return nil, fmt.Errorf("no nodes found in cluster")
	}

	// Use a map to track unique CIDRs
	cidrMap := make(map[string]bool)

	// Collect CIDRs from all nodes
	for _, node := range nodes.Items {
		// Check single PodCIDR field
		if node.Spec.PodCIDR != "" {
			cidrMap[node.Spec.PodCIDR] = true
		}

		// Check PodCIDRs slice (for dual-stack or multi-CIDR setups)
		for _, cidr := range node.Spec.PodCIDRs {
			if cidr != "" {
				cidrMap[cidr] = true
			}
		}
	}

	// Convert map keys to slice
	var cidrs []string
	for cidr := range cidrMap {
		cidrs = append(cidrs, cidr)
	}

	if len(cidrs) == 0 {
		return nil, fmt.Errorf("no pod CIDRs found in any node specification")
	}

	return cidrs, nil
}

// IsEndpointFromLocalPodNet checks if the given endpoints contain addresses that belong
// to any of the provided pod CIDR ranges. It returns true if at least one endpoint
// address (including not-ready addresses) is found within any of the pod networks.
func IsEndpointFromLocalPodNet(endpoints *corev1.Endpoints, podCIDRs []string) (bool, error) {
	if endpoints == nil || len(podCIDRs) == 0 {
		return false, nil
	}

	// Parse all CIDRs into network objects
	var podNetworks []*net.IPNet
	for _, cidr := range podCIDRs {
		_, podNet, err := net.ParseCIDR(cidr)
		if err != nil {
			return false, fmt.Errorf("failed to parse pod CIDR %s: %w", cidr, err)
		}
		podNetworks = append(podNetworks, podNet)
	}

	// Check if any endpoint addresses are in any of the local pod CIDRs
	for _, subset := range endpoints.Subsets {
		// Check ready addresses
		for _, addr := range subset.Addresses {
			if isIPInNetworks(addr.IP, podNetworks) {
				return true, nil
			}
		}
		// Also check NotReadyAddresses in case pods are starting up
		for _, addr := range subset.NotReadyAddresses {
			if isIPInNetworks(addr.IP, podNetworks) {
				return true, nil
			}
		}
	}

	return false, nil
}

// isIPInNetworks checks if the given IP string is contained within any of the provided networks.
func isIPInNetworks(ipStr string, networks []*net.IPNet) bool {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}

	for _, network := range networks {
		if network.Contains(ip) {
			return true
		}
	}
	return false
}
