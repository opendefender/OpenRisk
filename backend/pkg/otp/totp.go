// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package otp

import (
	"bytes"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"image/jpeg"
	"time"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
	"github.com/skip2/go-qrcode"
)

// GenerateTOTPSecret generates a new TOTP secret (32 bytes)
// Returns the secret string (base32 encoded)
func GenerateTOTPSecret() (string, error) {
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      "OpenRisk",
		AccountName: "openrisk-user",
		Period:      30,
		SecretSize:  32,
		Digits:      otp.DigitsSix,
		Algorithm:   otp.AlgorithmSHA1,
	})
	if err != nil {
		return "", fmt.Errorf("failed to generate TOTP secret: %w", err)
	}

	return key.Secret(), nil
}

// GenerateTOTPSecret2 generates a TOTP secret for a specific user/email
// Returns (secret, error)
func GenerateTOTPSecret2(userEmail string) (string, error) {
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      "OpenRisk GRC",
		AccountName: userEmail,
		Period:      30,
		SecretSize:  32,
		Digits:      otp.DigitsSix,
		Algorithm:   otp.AlgorithmSHA1,
	})
	if err != nil {
		return "", fmt.Errorf("failed to generate TOTP: %w", err)
	}

	return key.Secret(), nil
}

// GetTOTPQRCode generates a QR code for TOTP setup
// Returns base64-encoded JPEG image of QR code
func GetTOTPQRCode(secret, userEmail string) (string, error) {
	// Generate otpauth:// URL
	key, err := otp.NewKeyFromURL(fmt.Sprintf(
		"otpauth://totp/OpenRisk:%s?secret=%s&issuer=OpenRisk",
		userEmail,
		secret,
	))
	if err != nil {
		return "", fmt.Errorf("failed to create OTP key: %w", err)
	}

	// Generate QR code image
	qrCode, err := qrcode.New(key.URL(), qrcode.Medium)
	if err != nil {
		return "", fmt.Errorf("failed to generate QR code: %w", err)
	}

	// Encode to JPEG
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, qrCode.Image(200), nil); err != nil {
		return "", fmt.Errorf("failed to encode QR code: %w", err)
	}

	// Return base64-encoded image
	return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

// VerifyTOTP verifies a TOTP code against a secret
// Tolerance is ±1 time step (30 seconds window)
func VerifyTOTP(secret, code string) bool {
	return totp.Validate(code, secret)
}

// VerifyTOTPWithCustomWindow verifies TOTP with custom time window
// window: number of time steps to check (default 1)
func VerifyTOTPWithCustomWindow(secret, code string, window uint) bool {
	// ValidateCustom takes (code, secret, timestamp, opts)
	valid, _ := totp.ValidateCustom(code, secret, time.Now(), totp.ValidateOpts{
		Period:    30,
		Skew:      window,
		Digits:    otp.DigitsSix,
		Algorithm: otp.AlgorithmSHA1,
	})
	return valid
}

// totpPeriod is the step length every enrolled authenticator uses.
const totpPeriod = 30

// MatchTOTPStep reports which time step a code belongs to, checking the current
// step and one on each side (the same ±30 s tolerance as VerifyTOTP).
//
// VerifyTOTP only answers yes or no, so a caller cannot tell a fresh code from
// one it already accepted (#849). The step is what lets it refuse a replay: a
// code is accepted only if its step is later than the last one accepted for
// that secret. Codes are compared in constant time.
func MatchTOTPStep(secret, code string, now time.Time) (int64, bool) {
	if len(code) != 6 {
		return 0, false
	}
	current := now.Unix() / totpPeriod
	matched, found := int64(0), false
	for _, step := range []int64{current - 1, current, current + 1} {
		want, err := totp.GenerateCodeCustom(secret, time.Unix(step*totpPeriod, 0).UTC(), totp.ValidateOpts{
			Period:    totpPeriod,
			Digits:    otp.DigitsSix,
			Algorithm: otp.AlgorithmSHA1,
		})
		if err != nil {
			return 0, false
		}
		// No early return: every step costs the same whether or not it matches.
		if subtle.ConstantTimeCompare([]byte(want), []byte(code)) == 1 {
			matched, found = step, true
		}
	}
	return matched, found
}

// backupCodeCount is how many single-use MFA backup codes are minted per user.
const backupCodeCount = 8

// backupCodeLength is the character length of each backup code.
const backupCodeLength = 12

// backupCodeAlphabet is a Crockford-style base32 alphabet (no 0/1/8/9 to avoid
// O/I/B/g confusion). Its length is 32, an exact divisor of 256, so mapping a
// uniform random byte with `b % 32` introduces no modulo bias.
const backupCodeAlphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZ234567"

// GenerateBackupCodes returns backupCodeCount single-use MFA backup codes, each
// backupCodeLength characters drawn uniformly from backupCodeAlphabet using
// crypto/rand.
//
// SECURITY: every code MUST be independently unpredictable. A previous
// implementation seeded a linear congruential generator with a hard-coded
// constant, which made every user's codes identical and derivable from the
// public source — a universal MFA bypass (CWE-330/338/798). Do not reintroduce
// any deterministic seed here.
func GenerateBackupCodes() ([]string, error) {
	// One CSPRNG read for all codes; len(alphabet)==32 divides 256 evenly, so
	// `b % 32` is unbiased.
	raw := make([]byte, backupCodeCount*backupCodeLength)
	if _, err := rand.Read(raw); err != nil {
		return nil, fmt.Errorf("failed to read CSPRNG for backup codes: %w", err)
	}

	codes := make([]string, backupCodeCount)
	for i := 0; i < backupCodeCount; i++ {
		code := make([]byte, backupCodeLength)
		for j := 0; j < backupCodeLength; j++ {
			code[j] = backupCodeAlphabet[raw[i*backupCodeLength+j]%byte(len(backupCodeAlphabet))]
		}
		codes[i] = string(code)
	}
	return codes, nil
}
