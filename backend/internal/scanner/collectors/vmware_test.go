// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: LicenseRef-OpenRisk-Commercial

package collectors

import (
	"context"
	"testing"

	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/govmomi/vim25"

	"github.com/opendefender/openrisk/internal/domain"
	scanner "github.com/opendefender/openrisk/internal/scanner"
)

// TestVMwareCollect drives the real govmomi collector against the bundled
// vCenter simulator (vcsim), which ships several VirtualMachine objects.
func TestVMwareCollect(t *testing.T) {
	simulator.Test(func(ctx context.Context, vc *vim25.Client) {
		assets := make(chan scanner.AssetDiscovery, 32)
		findings := make(chan scanner.FindingDiscovery, 32)
		errs := make(chan error, 32)

		collectVMs(ctx, vc, assets, findings, errs)
		close(assets)
		close(findings)
		close(errs)

		for e := range errs {
			t.Fatalf("unexpected error: %v", e)
		}

		var n int
		for a := range assets {
			n++
			if a.Type != domain.AssetTypeVM {
				t.Fatalf("expected VM asset, got %q", a.Type)
			}
			if a.ExternalID == "" || a.Name == "" {
				t.Fatalf("VM asset missing id/name: %+v", a)
			}
		}
		if n == 0 {
			t.Fatal("expected the simulator to yield at least one VM")
		}
	})
}
