// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package logsafety

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/opendefender/openrisk/pkg/ticketing"
)

type doerFunc func(*http.Request) (*http.Response, error)

func (f doerFunc) Do(r *http.Request) (*http.Response, error) { return f(r) }

// Rule 6 at run time (#1317): a failing integration call returns an error that
// callers log with %v, so the error text must hold no credential, in clear or
// as the Basic header it was sent in.
func TestIntegrationFailure_ErrorTextCarriesNoCredential(t *testing.T) {
	const token, pass = "jira-api-token-s3cr3t-value", "snow-passw0rd-s3cr3t-value"
	basicJira := base64.StdEncoding.EncodeToString([]byte("a@b.co:" + token))
	basicSnow := base64.StdEncoding.EncodeToString([]byte("svc:" + pass))

	cases := []struct {
		name     string
		provider string
		creds    map[string]string
		forbid   []string
	}{
		{"jira", ticketing.ProviderJira, map[string]string{"email": "a@b.co", "api_token": token}, []string{token, basicJira}},
		{"servicenow", ticketing.ProviderServiceNow, map[string]string{"username": "svc", "password": pass}, []string{pass, basicSnow}},
	}
	failures := map[string]doerFunc{
		"401 from the remote": func(r *http.Request) (*http.Response, error) {
			// A hostile remote echoing the request it received.
			return &http.Response{StatusCode: 401, Status: "401", Body: io.NopCloser(strings.NewReader(r.Header.Get("Authorization")))}, nil
		},
		"transport error": func(*http.Request) (*http.Response, error) { return nil, errors.New("connection refused") },
	}
	for _, tc := range cases {
		for fname, doer := range failures {
			t.Run(tc.name+"/"+fname, func(t *testing.T) {
				p, ok := ticketing.ProviderFor(tc.provider)
				if !ok {
					t.Fatalf("no provider %s", tc.provider)
				}
				var buf bytes.Buffer
				log.SetOutput(&buf)
				defer log.SetOutput(os.Stderr)

				_, err := p.Create(context.Background(), ticketing.CreateRequest{
					BaseURL: "https://tickets.example.test", Credentials: tc.creds,
					ProjectOrTable: "incident", Summary: "s", HTTP: doer,
				})
				if err == nil {
					t.Fatal("want a failure")
				}
				log.Printf("ticketing failed: %v", err) // what a caller does
				for _, secret := range tc.forbid {
					if strings.Contains(buf.String(), secret) {
						t.Fatalf("the log line contains a credential: %q", buf.String())
					}
				}
			})
		}
	}
}
