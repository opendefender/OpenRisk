// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: LicenseRef-OpenRisk-Commercial
// This file is part of the OpenRisk Enterprise Edition and is NOT covered by the
// AGPL; it is licensed under the OpenRisk Commercial License (see LICENSE.commercial).

package collectors

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/opendefender/openrisk/internal/domain"
	scanner "github.com/opendefender/openrisk/internal/scanner"
)

// Kubernetes is a real client-go CloudCollector. It talks to a cluster's API
// server with a ServiceAccount bearer token and enumerates Nodes (Server assets)
// and Pods (Container assets), flagging pods that run a privileged container.
type Kubernetes struct{}

// NewKubernetes returns the Kubernetes collector.
func NewKubernetes() scanner.CloudCollector { return Kubernetes{} }

func (Kubernetes) Collect(ctx context.Context, cfg scanner.ScanConfig, assets chan<- scanner.AssetDiscovery, findings chan<- scanner.FindingDiscovery, errs chan<- error) {
	restCfg := &rest.Config{
		Host:        cfg.Credentials["api_server"],
		BearerToken: cfg.Credentials["token"],
	}
	if ca := cfg.Credentials["ca_cert"]; ca != "" {
		restCfg.TLSClientConfig.CAData = []byte(ca)
	} else {
		// No CA provided → skip verification (self-signed clusters). Documented
		// on the connector; production configs should supply ca_cert.
		restCfg.TLSClientConfig.Insecure = true
	}
	clientset, err := kubernetes.NewForConfig(restCfg)
	if err != nil {
		errs <- fmt.Errorf("kubernetes: client: %w", err)
		return
	}
	collectK8s(ctx, clientset, assets, findings, errs)
}

// collectK8s enumerates nodes and pods from any kubernetes.Interface, so it can
// be tested against the client-go fake clientset.
func collectK8s(ctx context.Context, cs kubernetes.Interface, assets chan<- scanner.AssetDiscovery, findings chan<- scanner.FindingDiscovery, errs chan<- error) {
	nodes, err := cs.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		errs <- fmt.Errorf("kubernetes: list nodes: %w", err)
	} else {
		for i := range nodes.Items {
			emitNode(nodes.Items[i], assets)
		}
	}

	pods, err := cs.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
	if err != nil {
		errs <- fmt.Errorf("kubernetes: list pods: %w", err)
		return
	}
	for i := range pods.Items {
		emitPod(pods.Items[i], assets, findings)
	}
}

func emitNode(n corev1.Node, assets chan<- scanner.AssetDiscovery) {
	info := n.Status.NodeInfo
	a := scanner.AssetDiscovery{
		ExternalID:  "k8s:node:" + string(n.UID),
		Name:        n.Name,
		Type:        domain.AssetTypeServer,
		CPE:         nodeOSCPE(info.OSImage),
		Tags:        []string{"kubernetes", "node", "kubelet:" + info.KubeletVersion},
		RawMetadata: map[string]any{"os_image": info.OSImage, "kernel": info.KernelVersion, "runtime": info.ContainerRuntimeVersion, "arch": info.Architecture},
	}
	if info.OSImage != "" {
		a.OS = ptr(info.OSImage)
		a.OSVersion = ptr(info.KernelVersion)
	}
	for _, addr := range n.Status.Addresses {
		if addr.Type == corev1.NodeInternalIP {
			a.IP = ptr(addr.Address)
		}
		if addr.Type == corev1.NodeHostName {
			a.Hostname = ptr(addr.Address)
		}
	}
	assets <- a
}

func emitPod(p corev1.Pod, assets chan<- scanner.AssetDiscovery, findings chan<- scanner.FindingDiscovery) {
	externalID := "k8s:pod:" + string(p.UID)
	image := ""
	if len(p.Spec.Containers) > 0 {
		image = p.Spec.Containers[0].Image
	}
	a := scanner.AssetDiscovery{
		ExternalID:  externalID,
		Name:        p.Namespace + "/" + p.Name,
		Type:        domain.AssetTypeContainer,
		CPE:         imageCPE(image),
		Environment: p.Namespace,
		Tags:        []string{"kubernetes", "pod", "ns:" + p.Namespace, strings.ToLower(string(p.Status.Phase))},
		RawMetadata: map[string]any{"namespace": p.Namespace, "node": p.Spec.NodeName, "image": image, "phase": string(p.Status.Phase)},
	}
	if p.Status.PodIP != "" {
		a.IP = ptr(p.Status.PodIP)
	}
	assets <- a

	for _, c := range p.Spec.Containers {
		if c.SecurityContext != nil && c.SecurityContext.Privileged != nil && *c.SecurityContext.Privileged {
			findings <- scanner.FindingDiscovery{
				Title:           "Privileged container",
				Description:     fmt.Sprintf("Pod %s/%s runs container %q in privileged mode, granting it host-level access.", p.Namespace, p.Name, c.Name),
				Severity:        scanner.SeverityHigh,
				Evidence:        "securityContext.privileged=true",
				RemediationHint: "Drop privileged mode; grant only the specific capabilities the workload needs.",
				Source:          "kubernetes",
				AssetExternalID: externalID,
			}
			break
		}
	}
}

func nodeOSCPE(osImage string) []string {
	l := strings.ToLower(osImage)
	switch {
	case strings.Contains(l, "ubuntu"):
		return []string{"cpe:2.3:o:canonical:ubuntu_linux"}
	case strings.Contains(l, "flatcar"), strings.Contains(l, "container-optimized"), strings.Contains(l, "coreos"):
		return []string{"cpe:2.3:o:linux:linux_kernel"}
	case strings.Contains(l, "red hat"), strings.Contains(l, "rhel"), strings.Contains(l, "centos"):
		return []string{"cpe:2.3:o:redhat:enterprise_linux"}
	case strings.Contains(l, "linux"):
		return []string{"cpe:2.3:o:linux:linux_kernel"}
	default:
		return nil
	}
}
