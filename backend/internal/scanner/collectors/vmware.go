// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: LicenseRef-OpenRisk-Commercial
// This file is part of the OpenRisk Enterprise Edition and is NOT covered by the
// AGPL; it is licensed under the OpenRisk Commercial License (see LICENSE.commercial).

package collectors

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/view"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/soap"

	"github.com/opendefender/openrisk/internal/domain"
	scanner "github.com/opendefender/openrisk/internal/scanner"
)

// VMware is a real govmomi CloudCollector. It logs in to a vCenter/ESXi endpoint
// and enumerates virtual machines (VM assets, guest OS → CPE), flagging VMs
// whose VMware Tools are missing or out of date.
type VMware struct{}

// NewVMware returns the VMware collector.
func NewVMware() scanner.CloudCollector { return VMware{} }

func (VMware) Collect(ctx context.Context, cfg scanner.ScanConfig, assets chan<- scanner.AssetDiscovery, findings chan<- scanner.FindingDiscovery, errs chan<- error) {
	u, err := soap.ParseURL(cfg.Credentials["url"])
	if err != nil {
		errs <- fmt.Errorf("vmware: parse url: %w", err)
		return
	}
	u.User = url.UserPassword(cfg.Credentials["username"], cfg.Credentials["password"])
	insecure := cfg.Credentials["insecure"] == "true"

	c, err := govmomi.NewClient(ctx, u, insecure)
	if err != nil {
		errs <- fmt.Errorf("vmware: connect: %w", err)
		return
	}
	defer func() { _ = c.Logout(context.Background()) }()

	collectVMs(ctx, c.Client, assets, findings, errs)
}

// collectVMs enumerates VirtualMachine objects from a vim25 client. Split out so
// it can be tested against the govmomi vcsim simulator.
func collectVMs(ctx context.Context, vc *vim25.Client, assets chan<- scanner.AssetDiscovery, findings chan<- scanner.FindingDiscovery, errs chan<- error) {
	m := view.NewManager(vc)
	v, err := m.CreateContainerView(ctx, vc.ServiceContent.RootFolder, []string{"VirtualMachine"}, true)
	if err != nil {
		errs <- fmt.Errorf("vmware: create view: %w", err)
		return
	}
	defer func() { _ = v.Destroy(context.Background()) }()

	var vms []mo.VirtualMachine
	if err := v.Retrieve(ctx, []string{"VirtualMachine"}, []string{"summary", "guest", "config"}, &vms); err != nil {
		errs <- fmt.Errorf("vmware: retrieve vms: %w", err)
		return
	}
	for _, vm := range vms {
		emitVM(vm, assets, findings)
	}
}

func emitVM(vm mo.VirtualMachine, assets chan<- scanner.AssetDiscovery, findings chan<- scanner.FindingDiscovery) {
	name := vm.Summary.Config.Name
	if name == "" {
		name = vm.Name
	}
	guestOS := vm.Summary.Config.GuestFullName
	moid := vm.Self.Value
	externalID := "vmware:vm:" + moid

	tags := []string{"vmware", "vm"}
	if ps := string(vm.Summary.Runtime.PowerState); ps != "" {
		tags = append(tags, strings.ToLower(ps))
	}
	a := scanner.AssetDiscovery{
		ExternalID:  externalID,
		Name:        name,
		Type:        domain.AssetTypeVM,
		CPE:         guestOSCPE(guestOS),
		Tags:        tags,
		RawMetadata: map[string]any{"guest_os": guestOS, "power_state": string(vm.Summary.Runtime.PowerState), "moid": moid},
	}
	if guestOS != "" {
		a.OS = ptr(guestOS)
	}
	if vm.Guest != nil && vm.Guest.IpAddress != "" {
		a.IP = ptr(vm.Guest.IpAddress)
		if vm.Guest.HostName != "" {
			a.Hostname = ptr(vm.Guest.HostName)
		}
	}
	assets <- a

	// VMware Tools hygiene.
	if vm.Guest != nil {
		switch vm.Guest.ToolsStatus {
		case "toolsNotInstalled":
			findings <- vmwareToolsFinding(name, externalID, "VMware Tools are not installed", scanner.SeverityMedium)
		case "toolsOld":
			findings <- vmwareToolsFinding(name, externalID, "VMware Tools are out of date", scanner.SeverityLow)
		}
	}
}

func vmwareToolsFinding(name, externalID, title string, sev string) scanner.FindingDiscovery {
	return scanner.FindingDiscovery{
		Title:           title,
		Description:     fmt.Sprintf("VM %q: %s. Up-to-date Tools are needed for reliable inventory, time sync and guest management.", name, strings.ToLower(title)),
		Severity:        sev,
		Evidence:        "guest.toolsStatus",
		RemediationHint: "Install or upgrade VMware Tools on the guest.",
		Source:          "vmware",
		AssetExternalID: externalID,
	}
}

func guestOSCPE(guest string) []string {
	l := strings.ToLower(guest)
	switch {
	case strings.Contains(l, "windows server"):
		return []string{"cpe:2.3:o:microsoft:windows_server"}
	case strings.Contains(l, "windows"):
		return []string{"cpe:2.3:o:microsoft:windows"}
	case strings.Contains(l, "ubuntu"):
		return []string{"cpe:2.3:o:canonical:ubuntu_linux"}
	case strings.Contains(l, "red hat"), strings.Contains(l, "rhel"), strings.Contains(l, "centos"):
		return []string{"cpe:2.3:o:redhat:enterprise_linux"}
	case strings.Contains(l, "linux"):
		return []string{"cpe:2.3:o:linux:linux_kernel"}
	default:
		return nil
	}
}
