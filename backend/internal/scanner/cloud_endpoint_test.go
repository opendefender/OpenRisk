// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package scanner

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opendefender/openrisk/internal/domain"
)

// #750: the endpoint of an in-process collector is checked when the config is
// saved, so a tenant cannot store a scan aimed at the API pod's own network.
func TestCloudScannerValidate_Endpoints(t *testing.T) {
	type tc struct {
		scanner Scanner
		creds   map[string]string
	}
	k8s := func(api string) tc {
		return tc{NewKubernetesScanner(nil), map[string]string{"api_server": api, "token": "t"}}
	}
	docker := func(host string) tc { return tc{NewDockerScanner(nil), map[string]string{"host": host}} }
	vmware := func(u string) tc {
		return tc{NewVMwareScanner(nil), map[string]string{"url": u, "username": "u", "password": "p"}}
	}
	ad := func(u string) tc {
		return tc{NewActiveDirectoryScanner(nil), map[string]string{"url": u, "bind_dn": "b", "password": "p", "base_dn": "d"}}
	}
	gh := func(u string) tc { return tc{NewGitHubScanner(nil), map[string]string{"token": "t", "base_url": u}} }
	gl := func(u string) tc { return tc{NewGitLabScanner(nil), map[string]string{"token": "t", "base_url": u}} }

	ok := []tc{
		k8s("https://k8s.example.com:6443"),
		k8s("k8s.example.com:6443"), // client-go reads a bare host as https
		docker("tcp://docker.example.com:2376"),
		vmware("https://vcenter.example.com/sdk"),
		vmware("vcenter.example.com"), // soap.ParseURL reads a bare host as https
		ad("ldaps://dc.example.com:636"),
		ad("ldap://dc.example.com"),
		gh(""), // empty base_url → api.github.com
		gh("https://ghe.example.com/api/v3/"),
		gl("https://gitlab.example.com"),
	}
	for _, c := range ok {
		cfg := ScanConfig{Provider: domain.ScannerProvider(c.scanner.Provider()), Credentials: c.creds}
		assert.NoError(t, c.scanner.Validate(context.Background(), cfg), "%s %v", c.scanner.Provider(), c.creds)
	}

	bad := []tc{
		k8s("https://127.0.0.1:6443"),
		k8s("https://kubernetes.localhost"),
		k8s("http://k8s.example.com"),
		docker("tcp://169.254.169.254:80"),
		docker("tcp://2130706433:2375"),
		docker("http://[::1]:2375"),
		vmware("https://169.254.169.254/sdk"),
		vmware("http://vcenter.example.com/sdk"),
		ad("ldapi:///var/run/slapd/ldapi"),
		ad("ldap://127.0.0.1:389"),
		ad("cldap://dc.example.com"),
		gh("https://169.254.169.254/"),
		gh("http://ghe.example.com/"),
		gl("https://localhost:8080"),
		gl("https://[fd00:ec2::254]/"),
	}
	for _, c := range bad {
		cfg := ScanConfig{Provider: domain.ScannerProvider(c.scanner.Provider()), Credentials: c.creds}
		err := c.scanner.Validate(context.Background(), cfg)
		require.Error(t, err, "%s %v", c.scanner.Provider(), c.creds)
		assert.ErrorIs(t, err, domain.ErrValidation)
	}
}

func TestCloudScannerValidate_DockerSocketNeedsOperatorOptIn(t *testing.T) {
	s := NewDockerScanner(nil)
	for _, host := range []string{"unix:///var/run/docker.sock", "/var/run/docker.sock"} {
		cfg := ScanConfig{Provider: domain.ProviderDocker, Credentials: map[string]string{"host": host}}

		t.Setenv(DockerSocketEnv, "")
		err := s.Validate(context.Background(), cfg)
		require.Error(t, err, host)
		assert.Contains(t, err.Error(), DockerSocketEnv)

		t.Setenv(DockerSocketEnv, "true")
		assert.NoError(t, s.Validate(context.Background(), cfg), host)
	}
}
