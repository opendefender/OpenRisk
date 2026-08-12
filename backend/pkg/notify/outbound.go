// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: LicenseRef-OpenRisk-Commercial
// This file is part of the OpenRisk Enterprise Edition and is NOT covered by the
// AGPL; it is licensed under the OpenRisk Commercial License (see LICENSE.commercial).

package notify

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// ---------------------------------------------------------------------------
// Generic outbound channels: a plain webhook and an HTTP SMS gateway.
//
// Slack and Teams cover the two chat tools most teams use; everything else —
// PagerDuty, an internal bus, a customer's own endpoint, an operator's SMS API —
// is served by these two. Both make a REAL request and return a REAL error: a
// non-2xx is never swallowed into a green "sent", because the whole point of a
// channel test is to find out that delivery does not work.
// ---------------------------------------------------------------------------

// WebhookPayload is the JSON body posted to a generic webhook. Stable by
// contract: receivers parse it, so fields are added but never renamed.
type WebhookPayload struct {
	Event     string            `json:"event"`
	Title     string            `json:"title"`
	Text      string            `json:"text"`
	Severity  string            `json:"severity,omitempty"`
	Facts     map[string]string `json:"facts,omitempty"`
	LinkURL   string            `json:"link_url,omitempty"`
	Timestamp string            `json:"timestamp"`
	Source    string            `json:"source"`
}

// PostWebhook delivers a message to an arbitrary HTTPS endpoint.
//
// When a secret is configured the body is signed with HMAC-SHA256 and sent as
// X-OpenRisk-Signature (hex, prefixed "sha256="), so the receiver can tell a
// genuine OpenRisk call from anyone who learned the URL. The signature covers
// the exact bytes sent.
func PostWebhook(ctx context.Context, url, secret string, msg ChatMessage, doer HTTPDoer) error {
	if strings.TrimSpace(url) == "" {
		return fmt.Errorf("no webhook URL configured")
	}
	facts := map[string]string{}
	for _, f := range msg.Facts {
		facts[f.Label] = f.Value
	}
	payload := WebhookPayload{
		Event:     "openrisk.alert",
		Title:     msg.Title,
		Text:      msg.Text,
		Severity:  msg.Severity,
		Facts:     facts,
		LinkURL:   msg.LinkURL,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Source:    "openrisk",
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal webhook payload: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build webhook request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "OpenRisk-Automation/1.0")
	if s := strings.TrimSpace(secret); s != "" {
		mac := hmac.New(sha256.New, []byte(s))
		mac.Write(body)
		req.Header.Set("X-OpenRisk-Signature", "sha256="+hex.EncodeToString(mac.Sum(nil)))
	}
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

// SMSConfig describes a generic HTTP SMS gateway.
//
// Deliberately field-name driven rather than vendor-specific: Orange, MTN,
// Twilio, Vonage and most African aggregators all expose "POST some JSON with a
// destination, a body and a sender", but none agree on the field names. Hard
// coding one vendor would serve one market; naming the fields serves all of
// them without pretending to support SDKs we have not tested.
type SMSConfig struct {
	GatewayURL  string
	APIKey      string
	Sender      string
	ToField     string // default "to"
	TextField   string // default "message"
	SenderField string // default "from"
}

// SendSMS posts one message per recipient and reports the first failure. It
// returns the number of recipients actually accepted by the gateway, so a
// partial delivery is visible rather than rounded up to success.
func SendSMS(ctx context.Context, cfg SMSConfig, recipients []string, text string, doer HTTPDoer) (sent int, err error) {
	if strings.TrimSpace(cfg.GatewayURL) == "" {
		return 0, fmt.Errorf("no SMS gateway URL configured")
	}
	if len(recipients) == 0 {
		return 0, fmt.Errorf("no SMS recipients configured")
	}
	toField := fallback(cfg.ToField, "to")
	textField := fallback(cfg.TextField, "message")
	senderField := fallback(cfg.SenderField, "from")

	for _, to := range recipients {
		to = strings.TrimSpace(to)
		if to == "" {
			continue
		}
		payload := map[string]string{toField: to, textField: text}
		if cfg.Sender != "" {
			payload[senderField] = cfg.Sender
		}
		body, mErr := json.Marshal(payload)
		if mErr != nil {
			return sent, fmt.Errorf("marshal SMS payload: %w", mErr)
		}
		req, rErr := http.NewRequestWithContext(ctx, http.MethodPost, cfg.GatewayURL, bytes.NewReader(body))
		if rErr != nil {
			return sent, fmt.Errorf("build SMS request: %w", rErr)
		}
		req.Header.Set("Content-Type", "application/json")
		if cfg.APIKey != "" {
			// Both shapes are common; sending each once is cheaper than making the
			// operator guess which one their gateway wants.
			req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
			req.Header.Set("X-API-Key", cfg.APIKey)
		}
		resp, dErr := httpDo(doer).Do(req)
		if dErr != nil {
			return sent, fmt.Errorf("send SMS to %s: %w", maskPhone(to), dErr)
		}
		status := resp.StatusCode
		resp.Body.Close()
		if status < 200 || status >= 300 {
			return sent, fmt.Errorf("SMS gateway returned status %d for %s", status, maskPhone(to))
		}
		sent++
	}
	if sent == 0 {
		return 0, fmt.Errorf("no usable SMS recipient")
	}
	return sent, nil
}

// maskPhone keeps enough of a number to identify it in an error without writing
// a full subscriber number into a log (RULE #6).
func maskPhone(s string) string {
	if len(s) <= 4 {
		return "***"
	}
	return "***" + s[len(s)-4:]
}

// SplitRecipients parses a comma/semicolon/whitespace separated recipient list.
func SplitRecipients(s string) []string {
	fields := strings.FieldsFunc(s, func(r rune) bool {
		return r == ',' || r == ';' || r == '\n' || r == ' ' || r == '\t'
	})
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		if f = strings.TrimSpace(f); f != "" {
			out = append(out, f)
		}
	}
	return out
}
