// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package cti

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type routeDoer struct {
	routes map[string]string // URL path → body
	urls   []string
}

func (d *routeDoer) Do(r *http.Request) (*http.Response, error) {
	d.urls = append(d.urls, r.URL.String())
	body, ok := d.routes[r.URL.Path]
	if !ok {
		return &http.Response{StatusCode: 404, Body: io.NopCloser(bytes.NewReader(nil))}, nil
	}
	return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body))}, nil
}

func TestProgramKeyword(t *testing.T) {
	assert.Equal(t, "7-Zip", ProgramKeyword("7-Zip 23.01 (x64)"))
	assert.Equal(t, "Notepad++", ProgramKeyword("Notepad++ (64-bit x64)"))
	assert.Equal(t, "Mozilla Firefox", ProgramKeyword("Mozilla Firefox v128.0.3 (x64 fr)"))
	assert.Equal(t, "", ProgramKeyword("(x64) 1.2.3"))
}

const cpeCandidates = `{"products":[
 {"cpe":{"cpeName":"cpe:2.3:a:7-zip:7-zip:23.01:*:*:*:*:*:*:*","deprecated":false}},
 {"cpe":{"cpeName":"cpe:2.3:a:someone:7-zip_theme:1.0:*:*:*:*:*:*:*","deprecated":false}},
 {"cpe":{"cpe":"","cpeName":"cpe:2.3:o:7-zip:7-zip_os:1:*:*:*:*:*:*:*","deprecated":false}}
]}`

func TestResolveCPE_Success(t *testing.T) {
	d := &routeDoer{routes: map[string]string{"/rest/json/cpes/2.0": cpeCandidates}}
	c := NewExternalClient(d, "")
	got, err := c.ResolveCPE(context.Background(), "Igor Pavlov", "7-Zip 23.01 (x64)")
	require.NoError(t, err)
	// The vendor "7-zip" is in the program name; the theme's vendor is not.
	assert.Equal(t, "7-zip:7-zip", got)
	assert.Contains(t, d.urls[0], "keywordSearch=7-Zip")
}

// A candidate whose vendor matches neither the publisher nor the name is not
// taken: no guess.
func TestResolveCPE_NotFound(t *testing.T) {
	d := &routeDoer{routes: map[string]string{"/rest/json/cpes/2.0": `{"products":[
	  {"cpe":{"cpeName":"cpe:2.3:a:acme:reporter:2.0:*:*:*:*:*:*:*","deprecated":false}}]}`}}
	got, err := NewExternalClient(d, "").ResolveCPE(context.Background(), "Contoso Ltd", "Reporter Pro 2.0")
	require.NoError(t, err)
	assert.Equal(t, "", got)
}

func TestFetchNVDByCPE(t *testing.T) {
	d := &routeDoer{routes: map[string]string{"/rest/json/cves/2.0": `{"vulnerabilities":[{"cve":{
	  "id":"CVE-2024-11477",
	  "descriptions":[{"lang":"en","value":"7-Zip Zstandard decompression integer underflow"}],
	  "metrics":{"cvssMetricV31":[{"cvssData":{"baseScore":7.8,"baseSeverity":"HIGH"}}]}}}]}`}}
	got, err := NewExternalClient(d, "").FetchNVDByCPE(context.Background(), "7-zip:7-zip", "23.01")
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "CVE-2024-11477", got[0].CVEID)
	assert.Equal(t, 7.8, got[0].CVSSV3)
	assert.Contains(t, d.urls[0], "virtualMatchString=cpe%3A2.3%3Aa%3A7-zip%3A7-zip%3A23.01")

	none, err := NewExternalClient(d, "").FetchNVDByCPE(context.Background(), "", "1")
	require.NoError(t, err)
	assert.Nil(t, none, "an unresolved program is never queried")
}

func TestCPEEscape(t *testing.T) {
	assert.Equal(t, "23.01", cpeEscape("23.01"))
	assert.Equal(t, `1\\x\:y\*`, cpeEscape(`1\x:y*`))
	assert.Equal(t, "10_build_7", cpeEscape("10 build 7"))
}
