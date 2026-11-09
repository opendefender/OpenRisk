// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package canonical

import (
	"net/url"
	"strings"
)

// secretParamNames are query or fragment parameter names that carry a
// credential: presigned-URL signatures (S3, GCS, Azure SAS), OAuth tokens and
// codes, API keys. Compared case-insensitively (ADR 0008 D3, ABSOLUTE RULE 6).
var secretParamNames = map[string]bool{
	"x-amz-signature": true, "x-amz-credential": true, "x-amz-security-token": true,
	"x-goog-signature": true, "x-goog-credential": true,
	"sig": true, "signature": true,
	"token": true, "access_token": true, "id_token": true, "refresh_token": true,
	"code": true, "key": true, "apikey": true, "api_key": true, "api-key": true,
	"secret": true, "client_secret": true, "password": true, "passwd": true, "pwd": true,
	"auth": true, "authorization": true, "jwt": true,
}

// secretParamSuffixes catch the variants a fixed list misses (`sessiontoken`,
// `X-Custom-Signature`, `db_password`).
var secretParamSuffixes = []string{"token", "signature", "secret", "password", "credential", "apikey", "api_key", "api-key"}

// credentialShaped reports whether s parses as a URL carrying userinfo or a
// signature or token parameter, in its query or its fragment (OAuth's implicit
// flow puts access_token in the fragment). A string that looks like a URL
// (contains "://") but does not parse is refused too: what cannot be inspected
// is not let through.
func credentialShaped(s string) bool {
	s = strings.TrimSpace(s)
	if !strings.ContainsAny(s, "@?#=") {
		return false
	}
	u, err := url.Parse(s)
	if err != nil {
		return strings.Contains(s, "://")
	}
	if u.User != nil {
		return true
	}
	// `user:secret@host` with no "//" parses as scheme "user" and an opaque
	// part; the userinfo is still there.
	if u.Opaque != "" && strings.Contains(u.Opaque, "@") {
		return true
	}
	return hasSecretParam(u.RawQuery) || hasSecretParam(u.Fragment)
}

func hasSecretParam(raw string) bool {
	for _, pair := range strings.FieldsFunc(raw, func(r rune) bool { return r == '&' || r == ';' }) {
		name, _, _ := strings.Cut(pair, "=")
		if unescaped, err := url.QueryUnescape(name); err == nil {
			name = unescaped
		}
		name = strings.ToLower(strings.TrimSpace(name))
		if secretParamNames[name] {
			return true
		}
		for _, suffix := range secretParamSuffixes {
			if strings.HasSuffix(name, suffix) {
				return true
			}
		}
	}
	return false
}

// validateExternalURL applies D6 and D3 to Evidence.external_url: an absolute
// http(s) URL with a host, and never credential-shaped (presigned links are
// most likely to be pasted here).
func validateExternalURL(field, s string) error {
	u, err := url.Parse(strings.TrimSpace(s))
	if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") {
		return invalid(field, "must be an absolute http or https URL")
	}
	if credentialShaped(s) {
		return invalid(field, "carries userinfo or a signature/token parameter")
	}
	return nil
}
