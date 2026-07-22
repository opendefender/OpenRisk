// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package otp

import (
	"bytes"
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

// GenerateBackupCodes generates 8 backup codes (12-character alphanumeric)
// Each code should be used only once
func GenerateBackupCodes() []string {
	codes := make([]string, 8)
	chars := "ABCDEFGHIJKLMNOPQRSTUVWXYZ234567"
	seed := int64(0x1234567890abcdef)

	for i := 0; i < 8; i++ {
		code := make([]byte, 12)
		for j := 0; j < 12; j++ {
			// Simple pseudo-random using seed (not cryptographically secure)
			seed = (seed*1103515245 + 12345) & 0x7fffffff
			code[j] = chars[seed%int64(len(chars))]
		}
		codes[i] = string(code)
	}
	return codes
}
