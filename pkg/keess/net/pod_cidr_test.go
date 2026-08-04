package net

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestGetPodCIDRs(t *testing.T) {
	tests := []struct {
		name        string
		nodes       []corev1.Node
		expected    []string
		expectError bool
	}{
		{
			name: "single node with PodCIDR",
			nodes: []corev1.Node{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "node1"},
					Spec: corev1.NodeSpec{
						PodCIDR: "10.244.0.0/24",
					},
				},
			},
			expected:    []string{"10.244.0.0/24"},
			expectError: false,
		},
		{
			name: "multiple nodes with different CIDRs",
			nodes: []corev1.Node{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "node1"},
					Spec: corev1.NodeSpec{
						PodCIDR: "10.244.0.0/24",
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "node2"},
					Spec: corev1.NodeSpec{
						PodCIDR: "10.244.1.0/24",
					},
				},
			},
			expected:    []string{"10.244.0.0/24", "10.244.1.0/24"},
			expectError: false,
		},
		{
			name: "node with PodCIDRs slice (dual-stack)",
			nodes: []corev1.Node{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "node1"},
					Spec: corev1.NodeSpec{
						PodCIDRs: []string{"10.244.0.0/24", "2001:db8::/64"},
					},
				},
			},
			expected:    []string{"10.244.0.0/24", "2001:db8::/64"},
			expectError: false,
		},
		{
			name: "duplicate CIDRs across nodes",
			nodes: []corev1.Node{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "node1"},
					Spec: corev1.NodeSpec{
						PodCIDR: "10.244.0.0/16",
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "node2"},
					Spec: corev1.NodeSpec{
						PodCIDR: "10.244.0.0/16", // Same CIDR
					},
				},
			},
			expected:    []string{"10.244.0.0/16"},
			expectError: false,
		},
		{
			name:        "no nodes",
			nodes:       []corev1.Node{},
			expected:    nil,
			expectError: true,
		},
		{
			name: "nodes without CIDRs",
			nodes: []corev1.Node{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "node1"},
					Spec:       corev1.NodeSpec{},
				},
			},
			expected:    nil,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create fake clientset with nodes
			clientset := fake.NewSimpleClientset()
			for _, node := range tt.nodes {
				_, err := clientset.CoreV1().Nodes().Create(context.Background(), &node, metav1.CreateOptions{})
				if err != nil {
					t.Fatalf("Failed to create test node: %v", err)
				}
			}

			// Test GetPodCIDRs
			cidrs, err := GetPodCIDRs(context.Background(), clientset.CoreV1())

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if len(cidrs) != len(tt.expected) {
				t.Errorf("Expected %d CIDRs, got %d", len(tt.expected), len(cidrs))
				return
			}

			// Convert to maps for comparison (order doesn't matter)
			expectedMap := make(map[string]bool)
			for _, cidr := range tt.expected {
				expectedMap[cidr] = true
			}

			actualMap := make(map[string]bool)
			for _, cidr := range cidrs {
				actualMap[cidr] = true
			}

			for expectedCIDR := range expectedMap {
				if !actualMap[expectedCIDR] {
					t.Errorf("Expected CIDR %s not found in result", expectedCIDR)
				}
			}

			for actualCIDR := range actualMap {
				if !expectedMap[actualCIDR] {
					t.Errorf("Unexpected CIDR %s found in result", actualCIDR)
				}
			}
		})
	}
}

func boolPtr(b bool) *bool {
	return &b
}

func TestIsEndpointFromLocalPodNet(t *testing.T) {
	tests := []struct {
		name           string
		endpointSlices []discoveryv1.EndpointSlice
		podCIDRs       []string
		expected       bool
		wantError      bool
	}{
		{
			name: "endpoint IP in pod CIDR",
			endpointSlices: []discoveryv1.EndpointSlice{
				{
					AddressType: discoveryv1.AddressTypeIPv4,
					Endpoints: []discoveryv1.Endpoint{
						{Addresses: []string{"10.244.1.5"}},
					},
				},
			},
			podCIDRs: []string{"10.244.0.0/16"},
			expected: true,
		},
		{
			name: "endpoint IP not in pod CIDR",
			endpointSlices: []discoveryv1.EndpointSlice{
				{
					AddressType: discoveryv1.AddressTypeIPv4,
					Endpoints: []discoveryv1.Endpoint{
						{Addresses: []string{"192.168.1.5"}},
					},
				},
			},
			podCIDRs: []string{"10.244.0.0/16"},
			expected: false,
		},
		{
			name: "endpoint IP in not-ready endpoint",
			endpointSlices: []discoveryv1.EndpointSlice{
				{
					AddressType: discoveryv1.AddressTypeIPv4,
					Endpoints: []discoveryv1.Endpoint{
						{
							Addresses: []string{"10.244.1.5"},
							Conditions: discoveryv1.EndpointConditions{
								Ready: boolPtr(false),
							},
						},
					},
				},
			},
			podCIDRs: []string{"10.244.0.0/16"},
			expected: true,
		},
		{
			name: "multiple CIDRs, matches second one",
			endpointSlices: []discoveryv1.EndpointSlice{
				{
					AddressType: discoveryv1.AddressTypeIPv4,
					Endpoints: []discoveryv1.Endpoint{
						{Addresses: []string{"10.245.1.5"}},
					},
				},
			},
			podCIDRs: []string{"10.244.0.0/16", "10.245.0.0/16"},
			expected: true,
		},
		{
			name: "dual-stack CIDRs with IPv6",
			endpointSlices: []discoveryv1.EndpointSlice{
				{
					AddressType: discoveryv1.AddressTypeIPv6,
					Endpoints: []discoveryv1.Endpoint{
						{Addresses: []string{"2001:db8::1"}},
					},
				},
			},
			podCIDRs: []string{"10.244.0.0/16", "2001:db8::/64"},
			expected: true,
		},
		{
			name:           "nil endpoint slices",
			endpointSlices: nil,
			podCIDRs:       []string{"10.244.0.0/16"},
			expected:       false,
		},
		{
			name: "empty podCIDRs",
			endpointSlices: []discoveryv1.EndpointSlice{
				{
					AddressType: discoveryv1.AddressTypeIPv4,
					Endpoints: []discoveryv1.Endpoint{
						{Addresses: []string{"10.244.1.5"}},
					},
				},
			},
			podCIDRs: []string{},
			expected: false,
		},
		{
			name: "invalid CIDR",
			endpointSlices: []discoveryv1.EndpointSlice{
				{
					AddressType: discoveryv1.AddressTypeIPv4,
					Endpoints: []discoveryv1.Endpoint{
						{Addresses: []string{"10.244.1.5"}},
					},
				},
			},
			podCIDRs:  []string{"invalid-cidr"},
			expected:  false,
			wantError: true,
		},
		{
			name: "invalid IP address",
			endpointSlices: []discoveryv1.EndpointSlice{
				{
					AddressType: discoveryv1.AddressTypeIPv4,
					Endpoints: []discoveryv1.Endpoint{
						{Addresses: []string{"invalid-ip"}},
					},
				},
			},
			podCIDRs: []string{"10.244.0.0/16"},
			expected: false,
		},
		{
			name: "match only in second endpoint slice",
			endpointSlices: []discoveryv1.EndpointSlice{
				{
					AddressType: discoveryv1.AddressTypeIPv4,
					Endpoints: []discoveryv1.Endpoint{
						{Addresses: []string{"192.168.1.5"}},
					},
				},
				{
					AddressType: discoveryv1.AddressTypeIPv4,
					Endpoints: []discoveryv1.Endpoint{
						{Addresses: []string{"10.244.1.5"}},
					},
				},
			},
			podCIDRs: []string{"10.244.0.0/16"},
			expected: true,
		},
		{
			name: "match only in last address of endpoint",
			endpointSlices: []discoveryv1.EndpointSlice{
				{
					AddressType: discoveryv1.AddressTypeIPv4,
					Endpoints: []discoveryv1.Endpoint{
						{Addresses: []string{"192.168.1.5", "172.16.0.1", "10.244.1.5"}},
					},
				},
			},
			podCIDRs: []string{"10.244.0.0/16"},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := IsEndpointFromLocalPodNet(tt.endpointSlices, tt.podCIDRs)

			if tt.wantError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}
