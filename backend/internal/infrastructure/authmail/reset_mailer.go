// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

// Package authmail renders and sends the account-security emails: password reset
// links, reset confirmations, and new-device sign-in notices.
//
// Every message exists in French and English. The language is the one the user
// was using when they asked, carried through the request — someone who has been
// working in French does not want a security email in English, least of all one
// telling them to click a link.
package authmail

import (
	"context"
	"fmt"
	"html"
	"strings"
	"time"
)

// Sender is the transport seam (satisfied by the project's email.Service).
type Sender interface {
	SendEmail(ctx context.Context, to, subject, body string) error
}

// Mailer renders and dispatches account-security email.
type Mailer struct {
	sender  Sender
	product string
}

// New builds a mailer over a transport.
func New(sender Sender) *Mailer {
	return &Mailer{sender: sender, product: "OpenRisk"}
}

// SendResetLink delivers the password-reset link.
func (m *Mailer) SendResetLink(ctx context.Context, to, fullName, link, locale string) error {
	if m == nil || m.sender == nil {
		return nil
	}
	c := resetLinkCopy(locale, displayName(fullName, locale), link)
	return m.sender.SendEmail(ctx, to, c.subject, m.render(c))
}

// SendResetConfirmation confirms the password changed and the sessions ended.
//
// This is a security notice, not a courtesy: it is how someone whose account was
// taken over finds out. It says plainly that every session was signed out and
// what to do if they did not do this.
func (m *Mailer) SendResetConfirmation(ctx context.Context, to, fullName, locale string) error {
	if m == nil || m.sender == nil {
		return nil
	}
	c := resetConfirmCopy(locale, displayName(fullName, locale))
	return m.sender.SendEmail(ctx, to, c.subject, m.render(c))
}

// SendNewSignInAlert warns about a sign-in from an unrecognised device.
func (m *Mailer) SendNewSignInAlert(ctx context.Context, to, fullName, ip, userAgent string, when time.Time, locale string) error {
	if m == nil || m.sender == nil {
		return nil
	}
	c := newSignInCopy(locale, displayName(fullName, locale), ip, userAgent, when)
	return m.sender.SendEmail(ctx, to, c.subject, m.render(c))
}

// ---------------------------------------------------------------------------
// Copy
// ---------------------------------------------------------------------------

type copyBlock struct {
	subject string
	heading string
	// paragraphs are pre-escaped plain sentences.
	paragraphs []string
	ctaLabel   string
	ctaURL     string
	// facts is an optional label/value list (used by the sign-in alert).
	facts [][2]string
	// footnote closes the message; typically the "if this wasn't you" line.
	footnote string
}

func resetLinkCopy(locale, name, link string) copyBlock {
	minutes := int(30) // mirrors domain.PasswordResetTTL
	if locale == "en" {
		return copyBlock{
			subject: "Reset your OpenRisk password",
			heading: "Reset your password",
			paragraphs: []string{
				fmt.Sprintf("Hello %s,", name),
				fmt.Sprintf("We received a request to reset the password for this OpenRisk account. Choose a new one using the button below. The link works once and expires in %d minutes.", minutes),
			},
			ctaLabel: "Choose a new password",
			ctaURL:   link,
			footnote: "If you didn't ask for this, you can ignore this email — your password stays as it is. Nothing changes until the link is used.",
		}
	}
	return copyBlock{
		subject: "Réinitialisez votre mot de passe OpenRisk",
		heading: "Réinitialisez votre mot de passe",
		paragraphs: []string{
			fmt.Sprintf("Bonjour %s,", name),
			fmt.Sprintf("Nous avons reçu une demande de réinitialisation du mot de passe de ce compte OpenRisk. Choisissez-en un nouveau via le bouton ci-dessous. Le lien est à usage unique et expire dans %d minutes.", minutes),
		},
		ctaLabel: "Choisir un nouveau mot de passe",
		ctaURL:   link,
		footnote: "Si vous n'êtes pas à l'origine de cette demande, ignorez cet e-mail : votre mot de passe reste inchangé. Rien ne change tant que le lien n'est pas utilisé.",
	}
}

func resetConfirmCopy(locale, name string) copyBlock {
	if locale == "en" {
		return copyBlock{
			subject: "Your OpenRisk password was changed",
			heading: "Your password was changed",
			paragraphs: []string{
				fmt.Sprintf("Hello %s,", name),
				"The password on this OpenRisk account has just been changed, and every active session was signed out. You'll need to sign in again on each of your devices.",
			},
			footnote: "If this wasn't you, your account is at risk: reset your password again immediately and contact your OpenRisk administrator.",
		}
	}
	return copyBlock{
		subject: "Votre mot de passe OpenRisk a été modifié",
		heading: "Votre mot de passe a été modifié",
		paragraphs: []string{
			fmt.Sprintf("Bonjour %s,", name),
			"Le mot de passe de ce compte OpenRisk vient d'être modifié, et toutes les sessions actives ont été déconnectées. Vous devrez vous reconnecter sur chacun de vos appareils.",
		},
		footnote: "Si vous n'êtes pas à l'origine de ce changement, votre compte est compromis : réinitialisez immédiatement votre mot de passe et contactez votre administrateur OpenRisk.",
	}
}

func newSignInCopy(locale, name, ip, userAgent string, when time.Time) copyBlock {
	stamp := when.UTC().Format("2006-01-02 15:04 UTC")
	if locale == "en" {
		return copyBlock{
			subject: "New sign-in to your OpenRisk account",
			heading: "New sign-in detected",
			paragraphs: []string{
				fmt.Sprintf("Hello %s,", name),
				"Your OpenRisk account was just signed into from a device we haven't seen before.",
			},
			facts: [][2]string{
				{"When", stamp},
				{"IP address", fallback(ip, "unknown")},
				{"Device", fallback(userAgent, "unknown")},
			},
			footnote: "If this was you, nothing to do. If it wasn't, change your password now and revoke the session from Settings → Sessions.",
		}
	}
	return copyBlock{
		subject: "Nouvelle connexion à votre compte OpenRisk",
		heading: "Nouvelle connexion détectée",
		paragraphs: []string{
			fmt.Sprintf("Bonjour %s,", name),
			"Une connexion à votre compte OpenRisk vient d'avoir lieu depuis un appareil que nous n'avions encore jamais vu.",
		},
		facts: [][2]string{
			{"Date", stamp},
			{"Adresse IP", fallback(ip, "inconnue")},
			{"Appareil", fallback(userAgent, "inconnu")},
		},
		footnote: "S'il s'agit de vous, il n'y a rien à faire. Sinon, changez votre mot de passe immédiatement et révoquez la session depuis Paramètres → Sessions.",
	}
}

// ---------------------------------------------------------------------------
// Rendering
// ---------------------------------------------------------------------------

// render produces the HTML body.
//
// Deliberately plain: table-free, inline styles, no external assets. Security
// mail has to render in the strictest corporate clients, and an image or a
// remote stylesheet is exactly what those strip. Everything interpolated is
// HTML-escaped — names and user agents are attacker-influenced strings.
func (m *Mailer) render(c copyBlock) string {
	var b strings.Builder
	b.WriteString(`<div style="font-family:-apple-system,Segoe UI,Roboto,Helvetica,Arial,sans-serif;max-width:520px;margin:0 auto;padding:32px 24px;color:#16181d;line-height:1.55">`)
	b.WriteString(`<div style="font-size:18px;font-weight:700;letter-spacing:-.01em;margin-bottom:24px">` + html.EscapeString(m.product) + `</div>`)
	b.WriteString(`<h1 style="font-size:21px;font-weight:700;margin:0 0 16px">` + html.EscapeString(c.heading) + `</h1>`)

	for _, p := range c.paragraphs {
		b.WriteString(`<p style="margin:0 0 14px;font-size:15px">` + html.EscapeString(p) + `</p>`)
	}

	if len(c.facts) > 0 {
		b.WriteString(`<div style="margin:18px 0;padding:14px 16px;background:#f5f6f8;border-radius:10px;font-size:14px">`)
		for _, f := range c.facts {
			b.WriteString(`<div style="margin:4px 0"><span style="color:#61656e">` + html.EscapeString(f[0]) + `</span> — <strong>` + html.EscapeString(f[1]) + `</strong></div>`)
		}
		b.WriteString(`</div>`)
	}

	if c.ctaURL != "" {
		safeURL := html.EscapeString(c.ctaURL)
		b.WriteString(`<p style="margin:24px 0"><a href="` + safeURL + `" style="display:inline-block;background:#2f6df6;color:#fff;text-decoration:none;padding:12px 22px;border-radius:10px;font-weight:600;font-size:15px">` + html.EscapeString(c.ctaLabel) + `</a></p>`)
		// Clients that strip buttons still need the link reachable.
		b.WriteString(`<p style="margin:0 0 14px;font-size:12.5px;color:#61656e;word-break:break-all">` + safeURL + `</p>`)
	}

	if c.footnote != "" {
		b.WriteString(`<p style="margin:22px 0 0;font-size:13px;color:#61656e;border-top:1px solid #e3e5ea;padding-top:16px">` + html.EscapeString(c.footnote) + `</p>`)
	}
	b.WriteString(`</div>`)
	return b.String()
}

func displayName(fullName, locale string) string {
	if n := strings.TrimSpace(fullName); n != "" {
		return n
	}
	if locale == "en" {
		return "there"
	}
	return "à vous"
}

func fallback(v, def string) string {
	if strings.TrimSpace(v) == "" {
		return def
	}
	return v
}
