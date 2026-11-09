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
// codes, API keys, passwords, session ids, SAML responses and CAS tickets. Compared case-insensitively, after
// unescaping and after stripping a trailing `[...]` (`token[]`) (ADR 0008 D3,
// ABSOLUTE RULE 6).
var secretParamNames = map[string]bool{
	"x-amz-signature": true, "x-amz-credential": true, "x-amz-security-token": true,
	"x-goog-signature": true, "x-goog-credential": true,
	"sig": true, "signature": true,
	"token": true, "access_token": true, "id_token": true, "refresh_token": true,
	"code": true, "key": true, "apikey": true, "api_key": true, "api-key": true,
	"secret": true, "client_secret": true, "password": true, "passwd": true, "pwd": true, "pass": true,
	"auth": true, "authorization": true, "jwt": true, "sessionid": true,
	"samlresponse": true, "ticket": true, "sid": true,
}

// secretParamSuffixes catch the variants a fixed list misses (`sessiontoken`,
// `X-Custom-Signature`, `db_password`, `secret_key`, `subscription-key`).
var secretParamSuffixes = []string{
	"token", "signature", "secret", "password", "credential", "apikey", "api_key", "api-key", "_key", "-key",
}

// maxURLNesting bounds how deep a URL carried inside another URL's parameter
// is followed. Deeper than this is refused: what is not inspected is not let
// through.
const maxURLNesting = 3

// credentialShaped reports whether s carries a credential in URL form:
// userinfo, or a signature or token parameter in the query or the fragment
// (OAuth's implicit flow and hash routers put them there), including inside a
// URL nested, plain or escaped, in a parameter value. A string that looks like
// a URL but does not parse is refused when it contains "://" or has the
// `user:pass@host` shape.
func credentialShaped(s string) bool {
	return credentialShapedAt(strings.TrimSpace(s), 0)
}

func credentialShapedAt(s string, depth int) bool {
	if !strings.ContainsAny(s, "@?#=") {
		return false
	}
	if depth > maxURLNesting {
		return true
	}
	u, err := url.Parse(s)
	if err != nil {
		return strings.Contains(s, "://") || userPassShape(s)
	}
	if u.User != nil {
		return true
	}
	// `user:secret@host` with no "//" parses as scheme "user" and an opaque
	// part; the userinfo is still there.
	if u.Opaque != "" && strings.Contains(u.Opaque, "@") {
		return true
	}
	if querySecret(u.RawQuery, depth) || querySecret(u.Fragment, depth) {
		return true
	}
	// A hash route (`#/cb?code=...`) carries its own query after the last '?'.
	if i := strings.LastIndexByte(u.Fragment, '?'); i >= 0 && querySecret(u.Fragment[i+1:], depth) {
		return true
	}
	return false
}

// userPassShape is `user:pass@host`: a ':' before an '@', with no '/' before
// that '@'.
func userPassShape(s string) bool {
	at := strings.IndexByte(s, '@')
	if at < 0 {
		return false
	}
	head := s[:at]
	return strings.Contains(head, ":") && !strings.Contains(head, "/")
}

// querySecret reports a secret-named parameter, or a parameter value that is
// itself credential-shaped (a URL nested in `next=`, escaped or not).
func querySecret(raw string, depth int) bool {
	for _, pair := range strings.FieldsFunc(raw, func(r rune) bool { return r == '&' || r == ';' }) {
		name, value, _ := strings.Cut(pair, "=")
		if secretParamName(name) {
			return true
		}
		decoded, state := unescapeFully(value)
		switch state {
		case unescapeUnstable:
			return true
		case unescapeFailed:
			// The value could not be decoded, so it cannot be inspected as a
			// URL. Refuse it when its escaped form still spells a nested URL,
			// a query or userinfo.
			lower := strings.ToLower(value)
			if strings.Contains(lower, "%3a%2f%2f") || strings.Contains(lower, "%3f") || strings.Contains(lower, "%40") {
				return true
			}
		}
		if strings.Contains(decoded, "://") || strings.ContainsAny(decoded, "?@") {
			if credentialShapedAt(decoded, depth+1) {
				return true
			}
		}
	}
	return false
}

type unescapeState int

const (
	unescapeOK       unescapeState = iota
	unescapeFailed                 // a malformed escape: the result is the last decodable form
	unescapeUnstable               // still changing after maxURLNesting rounds
)

// unescapeFully query-unescapes s until it stops changing, so a double-escaped
// `%253Ftoken%253D` is read as `?token=`.
func unescapeFully(s string) (string, unescapeState) {
	for i := 0; i <= maxURLNesting; i++ {
		decoded, err := url.QueryUnescape(s)
		if err != nil {
			return s, unescapeFailed
		}
		if decoded == s {
			return s, unescapeOK
		}
		s = decoded
	}
	return s, unescapeUnstable
}

func secretParamName(name string) bool {
	decoded, state := unescapeFully(name)
	if state != unescapeOK {
		// A parameter name with a malformed or ever-deeper escape (`token%zz`)
		// has no legitimate use and cannot be compared: fail closed.
		return true
	}
	name = strings.ToLower(strings.TrimSpace(decoded))
	if i := strings.IndexByte(name, '['); i > 0 && strings.HasSuffix(name, "]") {
		name = name[:i]
	}
	if secretParamNames[name] {
		return true
	}
	for _, suffix := range secretParamSuffixes {
		if strings.HasSuffix(name, suffix) {
			return true
		}
	}
	return false
}

// proseCarriesCredentialURL checks free text token by token: only a token that
// contains "://" is read as a URL, so an e-mail address or a "key: value" in a
// sentence is never mistaken for userinfo.
func proseCarriesCredentialURL(text string) bool {
	for _, token := range strings.Fields(text) {
		if strings.Contains(token, "://") && credentialShaped(token) {
			return true
		}
	}
	return false
}

// maxExternalURL caps Evidence.external_url.
const maxExternalURL = 2048

// validateExternalURL applies D6 and D3 to Evidence.external_url: an absolute
// http(s) URL with a host, and never credential-shaped (presigned links are
// most likely to be pasted here).
func validateExternalURL(field, s string) error {
	if err := maxLen(field, s, maxExternalURL); err != nil {
		return err
	}
	u, err := url.Parse(strings.TrimSpace(s))
	if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") {
		return invalid(field, "must be an absolute http or https URL")
	}
	if credentialShaped(s) {
		return invalid(field, "carries userinfo or a signature/token parameter")
	}
	return nil
}
