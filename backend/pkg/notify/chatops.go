// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: LicenseRef-OpenRisk-Commercial
// This file is part of the OpenRisk Enterprise Edition and is NOT covered by the
// AGPL; it is licensed under the OpenRisk Commercial License (see LICENSE.commercial).

package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// chatops.go — dependency-free webhook posters for Slack and Microsoft Teams.
// Both make REAL HTTP POSTs to an incoming-webhook URL; an empty URL or a
// non-2xx response returns a real error (never a silent success). They have no
// dependency on the domain layer so they stay reusable by the automation engine
// and the notification module alike, and are unit-tested via httptest.

// HTTPDoer is the seam for real requests (*http.Client) and httptest.
type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

// ChatMessage is a channel-agnostic alert rendered differently per platform.
type ChatMessage struct {
	Title string
	Text  string
	// Severity colours the accent (critical|high|medium|low). Empty = neutral.
	Severity string
	// Facts are key/value rows shown under the message (CVE, asset, due date…).
	Facts []ChatFact
	// LinkText/LinkURL render an optional "open in OpenRisk" action.
	LinkText string
	LinkURL  string
}

// ChatFact is one labelled value in a message.
type ChatFact struct {
	Label string
	Value string
}

func severityColor(severity string) string {
	switch strings.ToLower(severity) {
	case "critical":
		return "D7263D" // red
	case "high":
		return "F46036" // orange
	case "medium":
		return "F5B700" // amber
	case "low":
		return "2E86AB" // blue
	default:
		return "5A6ACF" // iris (brand)
	}
}

func httpDo(doer HTTPDoer) HTTPDoer {
	if doer != nil {
		return doer
	}
	return &http.Client{Timeout: 15 * time.Second}
}

// PostSlack sends a message to a Slack incoming webhook (attachment format).
func PostSlack(ctx context.Context, webhookURL string, msg ChatMessage, doer HTTPDoer) error {
	if strings.TrimSpace(webhookURL) == "" {
		return fmt.Errorf("slack webhook URL not configured")
	}
	fields := make([]map[string]interface{}, 0, len(msg.Facts))
	for _, f := range msg.Facts {
		fields = append(fields, map[string]interface{}{"title": f.Label, "value": f.Value, "short": true})
	}
	att := map[string]interface{}{
		"color":  "#" + severityColor(msg.Severity),
		"title":  msg.Title,
		"text":   msg.Text,
		"fields": fields,
		"footer": "OpenRisk Automation",
		"ts":     time.Now().Unix(),
	}
	if msg.LinkURL != "" {
		att["title_link"] = msg.LinkURL
	}
	payload := map[string]interface{}{
		"username":    "OpenRisk",
		"icon_emoji":  ":rotating_light:",
		"text":        msg.Title,
		"attachments": []interface{}{att},
	}
	return postJSON(ctx, webhookURL, payload, doer)
}

// PostTeams sends a message to a Microsoft Teams incoming webhook (MessageCard
// format — the format accepted by Teams "Incoming Webhook" connectors).
func PostTeams(ctx context.Context, webhookURL string, msg ChatMessage, doer HTTPDoer) error {
	if strings.TrimSpace(webhookURL) == "" {
		return fmt.Errorf("teams webhook URL not configured")
	}
	facts := make([]map[string]string, 0, len(msg.Facts))
	for _, f := range msg.Facts {
		facts = append(facts, map[string]string{"name": f.Label, "value": f.Value})
	}
	section := map[string]interface{}{
		"activityTitle": msg.Title,
		"text":          msg.Text,
		"markdown":      true,
	}
	if len(facts) > 0 {
		section["facts"] = facts
	}
	card := map[string]interface{}{
		"@type":      "MessageCard",
		"@context":   "https://schema.org/extensions",
		"themeColor": severityColor(msg.Severity),
		"summary":    msg.Title,
		"title":      msg.Title,
		"sections":   []interface{}{section},
	}
	if msg.LinkURL != "" {
		card["potentialAction"] = []interface{}{
			map[string]interface{}{
				"@type": "OpenUri",
				"name":  fallback(msg.LinkText, "Open in OpenRisk"),
				"targets": []interface{}{
					map[string]string{"os": "default", "uri": msg.LinkURL},
				},
			},
		}
	}
	return postJSON(ctx, webhookURL, card, doer)
}

func fallback(v, def string) string {
	if strings.TrimSpace(v) == "" {
		return def
	}
	return v
}

func postJSON(ctx context.Context, url string, payload interface{}, doer HTTPDoer) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal webhook payload: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build webhook request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := httpDo(doer).Do(req)
	if err != nil {
		return fmt.Errorf("post webhook: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}
	return nil
}
