// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: LicenseRef-OpenRisk-Commercial
// This file is part of the OpenRisk Enterprise Edition and is NOT covered by the
// AGPL; it is licensed under the OpenRisk Commercial License (see LICENSE.commercial).

package handler

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/oauth2"
)

// Microsoft identity checks (#1154).
//
// The ID token is read from the token endpoint's response, which the server
// received directly over TLS. OIDC Core 3.1.3.7 lets a client rely on that
// channel instead of the token signature; what is checked here is that the token
// was issued by the configured directory, for this application, is current, and
// names the user Graph then describes.

// azureTenant is OAUTH2_AZURE_TENANT_ID, trimmed: a tenant ID or a domain.
var azureTenant string

// azureClockSkew tolerates a small clock difference with Microsoft.
const azureClockSkew = 5 * time.Minute

var guidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// azureIDClaims are the ID token claims sign-in relies on.
type azureIDClaims struct {
	TenantID string
	ObjectID string
}

func verifyAzureIDToken(ctx context.Context, token *oauth2.Token) (*azureIDClaims, error) {
	raw, _ := token.Extra("id_token").(string)
	if raw == "" {
		return nil, errors.New("azure: token response carries no ID token")
	}
	cfg := providerConfig("azure")
	if cfg == nil || cfg.ClientID == "" {
		return nil, errors.New("azure: sign-in is not configured")
	}
	tenantID, err := resolveAzureTenantID(ctx)
	if err != nil {
		return nil, err
	}

	claims := jwt.MapClaims{}
	if _, _, err := jwt.NewParser().ParseUnverified(raw, claims); err != nil {
		return nil, fmt.Errorf("azure: malformed ID token: %w", err)
	}

	tid, _ := claims["tid"].(string)
	if !strings.EqualFold(tid, tenantID) {
		return nil, errors.New("azure: ID token comes from another directory")
	}
	if iss, _ := claims.GetIssuer(); iss != azureLoginBase+"/"+tid+"/v2.0" {
		return nil, errors.New("azure: unexpected ID token issuer")
	}
	aud, err := claims.GetAudience()
	if err != nil || !containsString(aud, cfg.ClientID) {
		return nil, errors.New("azure: ID token is not for this application")
	}
	exp, err := claims.GetExpirationTime()
	if err != nil || exp == nil || time.Now().After(exp.Add(azureClockSkew)) {
		return nil, errors.New("azure: ID token is expired")
	}
	oid, _ := claims["oid"].(string)
	if oid == "" {
		return nil, errors.New("azure: ID token names no user")
	}
	return &azureIDClaims{TenantID: tid, ObjectID: oid}, nil
}

func containsString(list []string, want string) bool {
	for _, v := range list {
		if v == want {
			return true
		}
	}
	return false
}

// azureTenantIDs caches the tenant ID a configured domain resolves to. It never
// expires: a domain only changes directory when it is moved, and a restart
// clears it. A stale entry fails closed on the tid check.
var azureTenantIDs sync.Map

// resolveAzureTenantID returns the configured tenant as a tenant ID. A domain is
// resolved once through the directory's discovery document, whose issuer names
// the tenant ID.
func resolveAzureTenantID(ctx context.Context) (string, error) {
	if guidPattern.MatchString(azureTenant) {
		return strings.ToLower(azureTenant), nil
	}
	if !isSpecificAzureTenant(azureTenant) {
		return "", errors.New("azure: no specific tenant configured")
	}
	key := strings.ToLower(azureTenant)
	if id, ok := azureTenantIDs.Load(key); ok {
		return id.(string), nil
	}

	var doc struct {
		Issuer string `json:"issuer"`
	}
	discovery := azureLoginBase + "/" + url.PathEscape(azureTenant) + "/v2.0/.well-known/openid-configuration"
	if err := fetchJSON(ctx, discovery, "", &doc); err != nil {
		return "", fmt.Errorf("azure: tenant discovery failed: %w", err)
	}
	// The issuer is <authority>/<tenant ID>/v2.0.
	rest, ok := strings.CutPrefix(doc.Issuer, azureLoginBase+"/")
	id, _, _ := strings.Cut(rest, "/")
	if !ok || !guidPattern.MatchString(id) {
		return "", errors.New("azure: tenant discovery returned no tenant ID")
	}
	id = strings.ToLower(id)
	azureTenantIDs.Store(key, id)
	return id, nil
}

// azureVerifiedDomains returns the directory's verified domains, lowercased.
func azureVerifiedDomains(ctx context.Context, token *oauth2.Token) (map[string]bool, error) {
	var org struct {
		Value []struct {
			VerifiedDomains []struct {
				Name string `json:"name"`
			} `json:"verifiedDomains"`
		} `json:"value"`
	}
	if err := fetchJSON(ctx, graphOrganizationURL, "Bearer "+token.AccessToken, &org); err != nil {
		return nil, err
	}
	domains := map[string]bool{}
	for _, o := range org.Value {
		for _, d := range o.VerifiedDomains {
			if name := strings.ToLower(strings.TrimSpace(d.Name)); name != "" {
				domains[name] = true
			}
		}
	}
	return domains, nil
}

// emailDomain returns the lowercased part after the last "@", or "".
func emailDomain(email string) string {
	i := strings.LastIndex(email, "@")
	if i < 0 {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(email[i+1:]))
}
