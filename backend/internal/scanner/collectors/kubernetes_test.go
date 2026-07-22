// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: LicenseRef-OpenRisk-Commercial

package collectors

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/opendefender/openrisk/internal/domain"
	scanner "github.com/opendefender/openrisk/internal/scanner"
)

// TestKubernetesCollect drives the real client-go collector against the official
// fake clientset seeded with a node and two pods (one privileged).
func TestKubernetesCollect(t *testing.T) {
	priv := true
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "node-1", UID: "n1"},
		Status: corev1.NodeStatus{
			NodeInfo:  corev1.NodeSystemInfo{OSImage: "Ubuntu 22.04.3 LTS", KubeletVersion: "v1.30.2", KernelVersion: "5.15.0"},
			Addresses: []corev1.NodeAddress{{Type: corev1.NodeInternalIP, Address: "10.0.0.5"}},
		},
	}
	safePod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default", UID: "p1"},
		Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "web", Image: "nginx:1.25"}}},
		Status:     corev1.PodStatus{Phase: corev1.PodRunning, PodIP: "10.1.0.9"},
	}
	privPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "agent", Namespace: "kube-system", UID: "p2"},
		Spec: corev1.PodSpec{Containers: []corev1.Container{{
			Name: "agent", Image: "agent:latest",
			SecurityContext: &corev1.SecurityContext{Privileged: &priv},
		}}},
		Status: corev1.PodStatus{Phase: corev1.PodRunning},
	}
	cs := fake.NewClientset(node, safePod, privPod)

	assets := make(chan scanner.AssetDiscovery, 32)
	findings := make(chan scanner.FindingDiscovery, 32)
	errs := make(chan error, 32)

	collectK8s(context.Background(), cs, assets, findings, errs)
	close(assets)
	close(findings)
	close(errs)

	for e := range errs {
		t.Fatalf("unexpected error: %v", e)
	}

	var nodes, pods int
	for a := range assets {
		switch a.Type {
		case domain.AssetTypeServer:
			nodes++
			assert.Equal(t, "10.0.0.5", *a.IP)
			assert.Contains(t, a.CPE, "cpe:2.3:o:canonical:ubuntu_linux")
		case domain.AssetTypeContainer:
			pods++
		}
	}
	assert.Equal(t, 1, nodes)
	assert.Equal(t, 2, pods)

	var gotFindings []scanner.FindingDiscovery
	for f := range findings {
		gotFindings = append(gotFindings, f)
	}
	require.Len(t, gotFindings, 1, "only the privileged pod raises a finding")
	assert.Equal(t, "k8s:pod:p2", gotFindings[0].AssetExternalID)
	assert.Equal(t, scanner.SeverityHigh, gotFindings[0].Severity)
}
