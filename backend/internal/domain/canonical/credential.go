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
		decoded, malformed, stable := unescapeFully(value)
		if !stable {
			return true
		}
		if malformed {
			// A stray '%' would make url.Parse fail before the nested URL is
			// read; escape it so the rest is inspected.
			decoded = escapeStrayPercent(decoded)
		}
		if strings.Contains(decoded, "://") || strings.ContainsAny(decoded, "?@#") {
			if credentialShapedAt(decoded, depth+1) {
				return true
			}
		}
	}
	return false
}

// unescapeFully query-unescapes s until it stops changing, so a double-escaped
// `%253Ftoken%253D` is read as `?token=`. Decoding is lenient: a malformed
// escape (`%zz`) is kept as it is and every valid one around it is still
// decoded, so a bad escape cannot hide the rest of the value from inspection.
// malformed reports that one was seen; stable is false when the value was still
// changing after maxURLNesting rounds.
func unescapeFully(s string) (decoded string, malformed, stable bool) {
	for i := 0; i <= maxURLNesting; i++ {
		next, bad := unescapeLenient(s)
		malformed = malformed || bad
		if next == s {
			return s, malformed, true
		}
		s = next
	}
	return s, malformed, false
}

// unescapeLenient is url.QueryUnescape that keeps a malformed escape literally
// instead of failing.
func unescapeLenient(s string) (string, bool) {
	var b strings.Builder
	b.Grow(len(s))
	malformed := false
	for i := 0; i < len(s); i++ {
		switch c := s[i]; {
		case c == '%' && i+2 < len(s) && isHex(s[i+1]) && isHex(s[i+2]):
			b.WriteByte(unhex(s[i+1])<<4 | unhex(s[i+2]))
			i += 2
		case c == '%':
			malformed = true
			b.WriteByte(c)
		case c == '+':
			b.WriteByte(' ')
		default:
			b.WriteByte(c)
		}
	}
	return b.String(), malformed
}

// escapeStrayPercent rewrites every '%' that does not start a valid escape as
// "%25".
func escapeStrayPercent(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		if s[i] == '%' && !(i+2 < len(s) && isHex(s[i+1]) && isHex(s[i+2])) {
			b.WriteString("%25")
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

func isHex(c byte) bool {
	return '0' <= c && c <= '9' || 'a' <= c && c <= 'f' || 'A' <= c && c <= 'F'
}

func unhex(c byte) byte {
	switch {
	case c >= 'a':
		return c - 'a' + 10
	case c >= 'A':
		return c - 'A' + 10
	default:
		return c - '0'
	}
}

func secretParamName(name string) bool {
	decoded, malformed, stable := unescapeFully(name)
	if malformed || !stable {
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
// sentence is never mistaken for userinfo. Punctuation around a token, as in
// "(https://…)" or a quoted link, is stripped before the URL is read.
func proseCarriesCredentialURL(text string) bool {
	for _, token := range strings.Fields(text) {
		token = strings.Trim(token, "()[]{}<>\"'.,;:!")
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
