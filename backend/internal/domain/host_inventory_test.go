// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package domain

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOSVEcosystem(t *testing.T) {
	cases := []struct {
		name                         string
		platform, version, src, want string
	}{
		{"debian", "debian", "12 (bookworm)", "deb", "Debian:12"},
		{"ubuntu lts", "ubuntu", "22.04.3 LTS (Jammy Jellyfish)", "deb", "Ubuntu:22.04:LTS"},
		{"ubuntu interim", "ubuntu", "23.10 (Mantic Minotaur)", "deb", "Ubuntu:23.10"},
		{"alpine", "alpine", "3.19.1", "apk", "Alpine:v3.19"},
		{"rocky", "rocky", "9.3 (Blue Onyx)", "rpm", "Rocky Linux:9"},
		{"alma", "almalinux", "9.4 (Seafoam Ocelot)", "rpm", "AlmaLinux:9"},
		// OSV's Red Hat ecosystem needs the repository CPE, which osquery does
		// not report; SUSE needs the exact PRETTY_NAME. Not checkable in v1.
		{"rhel", "rhel", "9.4 (Plow)", "rpm", ""},
		{"suse", "sles", "15-SP5", "rpm", ""},
		// Windows programs go to NVD, never to OSV.
		{"windows", "windows", "10.0.22631", "programs", ""},
		// A source that does not belong to the platform is not guessed.
		{"rpm on debian", "debian", "12", "rpm", ""},
		{"no version", "ubuntu", "", "deb", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, OSVEcosystem(c.platform, c.version, c.src))
		})
	}
}

func TestHostKey(t *testing.T) {
	assert.Equal(t, "srv-core-01", HostKey("SRV-Core-01.corp.local."))
	assert.Equal(t, "srv-core-01", HostKey("  srv-core-01 "))
	assert.Equal(t, "", HostKey("   "))
}

func TestNormalisePackages(t *testing.T) {
	long := strings.Repeat("x", 400)
	in := []HostPackageInput{
		{Name: " openssl ", Version: "3.0.7-27.el9", Source: "DEB"},
		{Name: "openssl", Version: "3.0.7-27.el9", Source: "deb"}, // duplicate
		{Name: "", Version: "1"},                                  // no name
		{Name: long, Version: long, Source: "deb"},
	}
	out := NormalisePackages(in)
	assert.Len(t, out, 2)
	assert.Equal(t, "openssl", out[0].Name)
	assert.Equal(t, "deb", out[0].Source)
	assert.Len(t, out[1].Name, maxPackageField)
	assert.Len(t, out[1].Version, maxPackageField)
}
