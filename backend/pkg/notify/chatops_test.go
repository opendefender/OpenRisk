// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1

package notify

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func sampleMessage() ChatMessage {
	return ChatMessage{
		Title:    "Critical CVE detected — CVE-2021-44228",
		Text:     "Log4Shell confirmed on web-01.",
		Severity: "critical",
		Facts: []ChatFact{
			{Label: "CVE", Value: "CVE-2021-44228"},
			{Label: "Asset", Value: "web-01"},
		},
		LinkText: "Open risk",
		LinkURL:  "https://app.openrisk.io/risks/42",
	}
}

func TestPostTeams_RealPostAndCardShape(t *testing.T) {
	var got map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &got)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	if err := PostTeams(context.Background(), srv.URL, sampleMessage(), srv.Client()); err != nil {
		t.Fatalf("PostTeams: %v", err)
	}
	if got["@type"] != "MessageCard" {
		t.Fatalf("expected MessageCard, got %v", got["@type"])
	}
	if got["themeColor"] != "D7263D" {
		t.Fatalf("expected critical themeColor D7263D, got %v", got["themeColor"])
	}
	if got["title"] != sampleMessage().Title {
		t.Fatalf("title not propagated: %v", got["title"])
	}
	// potentialAction present because a link was supplied.
	if _, ok := got["potentialAction"]; !ok {
		t.Fatalf("expected potentialAction for the link")
	}
}

func TestPostSlack_RealPostAndAttachment(t *testing.T) {
	var got map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &got)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	if err := PostSlack(context.Background(), srv.URL, sampleMessage(), srv.Client()); err != nil {
		t.Fatalf("PostSlack: %v", err)
	}
	atts, ok := got["attachments"].([]interface{})
	if !ok || len(atts) != 1 {
		t.Fatalf("expected one attachment, got %v", got["attachments"])
	}
	att := atts[0].(map[string]interface{})
	if !strings.HasPrefix(att["color"].(string), "#") {
		t.Fatalf("attachment color should be hex: %v", att["color"])
	}
}

func TestPostTeams_EmptyURL(t *testing.T) {
	if err := PostTeams(context.Background(), "", sampleMessage(), nil); err == nil {
		t.Fatal("expected error for empty webhook URL")
	}
}

func TestPostSlack_Non2xxIsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer srv.Close()
	if err := PostSlack(context.Background(), srv.URL, sampleMessage(), srv.Client()); err == nil {
		t.Fatal("expected error on non-2xx response")
	}
}
