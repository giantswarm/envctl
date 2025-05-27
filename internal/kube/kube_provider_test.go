package kube

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestDetermineProviderFromNode(t *testing.T) {
	tests := []struct {
		name         string
		node         *corev1.Node
		wantProvider string
	}{
		{
			name:         "aws via providerID",
			node:         &corev1.Node{Spec: corev1.NodeSpec{ProviderID: "aws:///us-east-1/i-12345"}},
			wantProvider: "aws",
		},
		{
			name:         "azure via providerID",
			node:         &corev1.Node{Spec: corev1.NodeSpec{ProviderID: "azure:///subscriptions/subid/resourceGroups/rg/providers/Microsoft.Compute/virtualMachines/vm-name"}},
			wantProvider: "azure",
		},
		{
			name:         "gcp via providerID",
			node:         &corev1.Node{Spec: corev1.NodeSpec{ProviderID: "gce://project-id/us-central1-a/instance-1"}},
			wantProvider: "gcp",
		},
		{
			name:         "vsphere via providerID",
			node:         &corev1.Node{Spec: corev1.NodeSpec{ProviderID: "vsphere://guid"}},
			wantProvider: "vsphere",
		},
		{
			name:         "openstack via providerID",
			node:         &corev1.Node{Spec: corev1.NodeSpec{ProviderID: "openstack:///id"}},
			wantProvider: "openstack",
		},
		{
			name:         "aws via eks label",
			node:         &corev1.Node{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"eks.amazonaws.com/nodegroup": "my-group"}}},
			wantProvider: "aws",
		},
		{
			name:         "aws via compute label",
			node:         &corev1.Node{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"alpha.kubernetes.io/instance-type": "m5.large", "topology.kubernetes.io/zone": "us-east-1a", "failure-domain.beta.kubernetes.io/zone": "us-east-1a", "node.kubernetes.io/instance-type": "m5.large", "beta.kubernetes.io/instance-type": "m5.large", "failure-domain.beta.kubernetes.io/region": "us-east-1", "topology.kubernetes.io/region": "us-east-1", "amazonaws.com/compute": "ec2"}}},
			wantProvider: "aws",
		},
		{
			name:         "azure via label",
			node:         &corev1.Node{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"kubernetes.azure.com/cluster": "aks-cluster"}}},
			wantProvider: "azure",
		},
		{
			name:         "gcp via gke label",
			node:         &corev1.Node{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"cloud.google.com/gke-nodepool": "pool-1"}}},
			wantProvider: "gcp",
		},
		{
			name:         "unknown provider - no ID, no matching labels",
			node:         &corev1.Node{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"foo": "bar"}}},
			wantProvider: "unknown",
		},
		{
			name:         "unknown provider - empty node spec and meta",
			node:         &corev1.Node{},
			wantProvider: "unknown",
		},
		{
			name:         "providerID present but not matched, fallback to unknown label",
			node:         &corev1.Node{Spec: corev1.NodeSpec{ProviderID: "somecloud://id"}, ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"foo": "bar"}}},
			wantProvider: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if gotProvider := determineProviderFromNode(tt.node); gotProvider != tt.wantProvider {
				t.Errorf("determineProviderFromNode() = %v, want %v", gotProvider, tt.wantProvider)
			}
		})
	}
}
