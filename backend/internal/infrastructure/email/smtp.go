// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package email

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"mime"
	"net"
	"net/smtp"
	"strings"
	"time"
)

// ---------------------------------------------------------------------------
// Honest mail transports.
//
// The transport this package shipped printed the message to stdout and returned
// nil. Every caller therefore believed its mail had been delivered — which is
// tolerable for a best-effort notification and is NOT tolerable for an
// invitation, where "we emailed them" is the whole user-visible outcome. A
// product that says an invitation was sent when nothing left the building is
// lying to an administrator about whether a colleague can get in.
//
// So there are two transports and no third state:
//
//   - Transport actually speaks SMTP and returns the server's real error.
//   - Unconfigured returns ErrNotConfigured, always.
//
// Callers that can degrade (scan notices, sign-in alerts) keep ignoring the
// error exactly as before. Callers that cannot — invitations — surface it.
// ---------------------------------------------------------------------------

// ErrNotConfigured reports that no mail transport is set up on this deployment.
var ErrNotConfigured = errors.New("email: no SMTP transport is configured")

// Unconfigured is the transport used when SMTP_HOST is unset. It never claims
// success.
type Unconfigured struct{}

// NewUnconfigured builds the honest no-op transport.
func NewUnconfigured() *Unconfigured { return &Unconfigured{} }

// SendEmail always fails, with a reason the caller can show a human.
func (*Unconfigured) SendEmail(context.Context, string, string, string) error {
	return ErrNotConfigured
}

// Config describes an SMTP endpoint.
type Config struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
	// StartTLS upgrades the connection after EHLO (port 587). When false and the
	// port is 465, the connection is TLS from the first byte.
	StartTLS bool
	Timeout  time.Duration
}

// Transport sends mail over SMTP.
type Transport struct{ cfg Config }

// NewTransport builds an SMTP transport. It does not dial: a mail server that
// is down must not stop the application from booting.
func NewTransport(cfg Config) *Transport {
	if cfg.Timeout <= 0 {
		cfg.Timeout = 15 * time.Second
	}
	if cfg.Port == 0 {
		cfg.Port = 587
	}
	if cfg.Port == 587 {
		cfg.StartTLS = true
	}
	return &Transport{cfg: cfg}
}

// SendEmail delivers one HTML message and returns whatever the server said.
func (t *Transport) SendEmail(ctx context.Context, to, subject, htmlBody string) error {
	if t == nil || strings.TrimSpace(t.cfg.Host) == "" {
		return ErrNotConfigured
	}
	addr := net.JoinHostPort(t.cfg.Host, fmt.Sprint(t.cfg.Port))

	dialer := &net.Dialer{Timeout: t.cfg.Timeout}
	var (
		conn net.Conn
		err  error
	)
	if t.cfg.Port == 465 {
		conn, err = tls.DialWithDialer(dialer, "tcp", addr, &tls.Config{ServerName: t.cfg.Host, MinVersion: tls.VersionTLS12})
	} else {
		conn, err = dialer.DialContext(ctx, "tcp", addr)
	}
	if err != nil {
		return fmt.Errorf("email: dial %s: %w", addr, err)
	}
	defer func() { _ = conn.Close() }()
	if dl, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(dl)
	} else {
		_ = conn.SetDeadline(time.Now().Add(t.cfg.Timeout))
	}

	client, err := smtp.NewClient(conn, t.cfg.Host)
	if err != nil {
		return fmt.Errorf("email: smtp handshake: %w", err)
	}
	defer func() { _ = client.Quit() }()

	if t.cfg.StartTLS {
		if ok, _ := client.Extension("STARTTLS"); ok {
			if err := client.StartTLS(&tls.Config{ServerName: t.cfg.Host, MinVersion: tls.VersionTLS12}); err != nil {
				return fmt.Errorf("email: starttls: %w", err)
			}
		}
	}
	if t.cfg.Username != "" {
		// PlainAuth refuses to send credentials over an unencrypted link, which is
		// the behaviour we want: a misconfigured deployment fails loudly here
		// rather than putting a password on the wire.
		if err := client.Auth(smtp.PlainAuth("", t.cfg.Username, t.cfg.Password, t.cfg.Host)); err != nil {
			return fmt.Errorf("email: auth: %w", err)
		}
	}
	if err := client.Mail(t.cfg.From); err != nil {
		return fmt.Errorf("email: from: %w", err)
	}
	if err := client.Rcpt(to); err != nil {
		return fmt.Errorf("email: rcpt: %w", err)
	}
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("email: data: %w", err)
	}
	if _, err := w.Write(t.message(to, subject, htmlBody)); err != nil {
		_ = w.Close()
		return fmt.Errorf("email: write: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("email: close: %w", err)
	}
	return nil
}

// message renders RFC 5322 headers plus the HTML body. The subject is
// Q-encoded so accented French copy survives the wire.
func (t *Transport) message(to, subject, htmlBody string) []byte {
	var b strings.Builder
	b.WriteString("From: " + t.cfg.From + "\r\n")
	b.WriteString("To: " + to + "\r\n")
	b.WriteString("Subject: " + mime.QEncoding.Encode("UTF-8", subject) + "\r\n")
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: text/html; charset=UTF-8\r\n")
	b.WriteString("Date: " + time.Now().Format(time.RFC1123Z) + "\r\n")
	b.WriteString("\r\n")
	b.WriteString(htmlBody)
	return []byte(b.String())
}

// FromEnv builds the transport a deployment configured, or the honest
// no-op when it configured none.
func FromEnv(host string, port int, username, password, from string) Service {
	if strings.TrimSpace(host) == "" {
		return NewUnconfigured()
	}
	return NewTransport(Config{Host: host, Port: port, Username: username, Password: password, From: from})
}
