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
	"github.com/opendefender/openrisk/pkg/netguard"
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
		// An operator opt-in is not a guard refusal (#1315).
		assert.ErrorIs(t, err, domain.ErrValidation)
		assert.False(t, netguard.IsDenied(err), host)

		t.Setenv(DockerSocketEnv, "true")
		assert.NoError(t, s.Validate(context.Background(), cfg), host)
	}
}

// #1315, GHSA-jxfm-p72g-w272: only a Google service-account key is accepted.
// Another credential type (external_account) makes the auth library fetch
// tenant-chosen URLs, and a service account's token_uri is where the SDK POSTs
// a signed assertion, so it must be one of Google's token endpoints.
func TestCloudScannerValidate_GCPServiceAccountKey(t *testing.T) {
	s := NewGCPScanner(nil)
	cfg := func(key string) ScanConfig {
		return ScanConfig{Provider: domain.ProviderGCP, Credentials: map[string]string{"service_account_json": key}}
	}
	for _, key := range []string{
		`{"type":"service_account","project_id":"p"}`, // no token_uri: Google's default
		`{"type":"service_account","token_uri":"https://oauth2.googleapis.com/token"}`,
		`{"type":"service_account","token_uri":"https://accounts.google.com/o/oauth2/token"}`,
		`{"type":"service_account","universe_domain":"googleapis.com"}`,
	} {
		assert.NoError(t, s.Validate(context.Background(), cfg(key)), key)
	}

	refused := []string{
		`   `,
		`not json`,
		`{"project_id":"p"}`,
		`{"type":"authorized_user"}`,
		`{"type":"external_account","credential_source":{"url":"http://169.254.169.254/computeMetadata/v1/"},"token_url":"https://attacker.example/token"}`,
		`{"type":"impersonated_service_account","service_account_impersonation_url":"https://attacker.example/"}`,
		`{"type":"service_account","universe_domain":"attacker.example"}`,
	}
	for _, uri := range []string{
		"http://169.254.169.254/computeMetadata/v1/",
		"https://169.254.169.254/token",
		"https://127.0.0.1:8080/token",
		"https://10.0.0.5/token",
		"https://[::1]/token",
		"http://oauth2.googleapis.com/token",
		"https://oauth2.googleapis.com.attacker.example/token",
		"https://attacker.example/token",
		"gopher://127.0.0.1:6379/_x",
	} {
		refused = append(refused, `{"type":"service_account","token_uri":"`+uri+`"}`)
	}
	for _, key := range refused {
		err := s.Validate(context.Background(), cfg(key))
		require.Error(t, err, key)
		assert.ErrorIs(t, err, domain.ErrValidation, key)
		for _, secret := range []string{"169.254.169.254", "attacker.example", "127.0.0.1"} {
			assert.NotContains(t, err.Error(), secret, "the 400 message must not echo the key's values")
		}
	}
}

// An empty key is refused by the required-credential check, before the SDK
// could fall back to the API pod's own identity.
func TestCloudScannerValidate_GCPEmptyKey(t *testing.T) {
	err := NewGCPScanner(nil).Validate(context.Background(), ScanConfig{
		Provider:    domain.ProviderGCP,
		Credentials: map[string]string{"service_account_json": "", "project_id": "p"},
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrValidation)
}
