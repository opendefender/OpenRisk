// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package domain

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

// ThemeMode is the interface theme a person chose.
type ThemeMode string

const (
	ThemeLight  ThemeMode = "light"
	ThemeDark   ThemeMode = "dark"
	ThemeSystem ThemeMode = "system"
)

// IsThemeMode reports whether m is a supported theme mode.
func IsThemeMode(m string) bool {
	switch ThemeMode(m) {
	case ThemeLight, ThemeDark, ThemeSystem:
		return true
	}
	return false
}

// phonePattern accepts an optional leading +, then digits with spaces, dots,
// dashes or parentheses — what people actually type — and nothing else.
var phonePattern = regexp.MustCompile(`^\+?[0-9 ().-]{4,32}$`)

// UserProfilePatch is a partial update of the caller's own profile. A nil field
// is left unchanged; an empty string clears it. FullName cannot be cleared.
// Identity and access fields (email, username, role, status) are deliberately
// absent: they cannot be reached through this type at all.
type UserProfilePatch struct {
	FullName   *string `json:"full_name,omitempty"`
	JobTitle   *string `json:"job_title,omitempty"`
	Phone      *string `json:"phone,omitempty"`
	Bio        *string `json:"bio,omitempty"`
	Timezone   *string `json:"timezone,omitempty"`
	Locale     *string `json:"locale,omitempty"`
	DateFormat *string `json:"date_format,omitempty"`
	ThemeMode  *string `json:"theme_mode,omitempty"`
}

// Normalize trims every provided field and validates it, returning the first
// violation as a validation error that names the field.
func (p *UserProfilePatch) Normalize() error {
	for _, f := range []*string{p.FullName, p.JobTitle, p.Phone, p.Bio, p.Timezone, p.Locale, p.DateFormat, p.ThemeMode} {
		if f != nil {
			*f = strings.TrimSpace(*f)
		}
	}
	if p.FullName != nil {
		if n := utf8.RuneCountInString(*p.FullName); n < 1 || n > 120 {
			return NewValidationError("full_name: must be between 1 and 120 characters")
		}
	}
	if p.JobTitle != nil && utf8.RuneCountInString(*p.JobTitle) > 80 {
		return NewValidationError("job_title: must be at most 80 characters")
	}
	if p.Phone != nil && *p.Phone != "" && !phonePattern.MatchString(*p.Phone) {
		return NewValidationError("phone: must be a phone number of at most 32 characters")
	}
	if p.Bio != nil && utf8.RuneCountInString(*p.Bio) > 500 {
		return NewValidationError("bio: must be at most 500 characters")
	}
	if p.Timezone != nil && *p.Timezone != "" && !IsTimezone(*p.Timezone) {
		return NewValidationError("timezone: unknown IANA time zone")
	}
	if p.Locale != nil && *p.Locale != "" && !IsEnabledLocale(*p.Locale) {
		return NewValidationError("locale: unsupported locale")
	}
	if p.DateFormat != nil && *p.DateFormat != "" && !IsDateFormat(*p.DateFormat) {
		return NewValidationError("date_format: must be one of DD/MM/YYYY, MM/DD/YYYY, YYYY-MM-DD")
	}
	if p.ThemeMode != nil && *p.ThemeMode != "" && !IsThemeMode(*p.ThemeMode) {
		return NewValidationError("theme_mode: must be one of light, dark, system")
	}
	return nil
}

// Columns maps the provided fields to users columns, for a partial UPDATE.
func (p *UserProfilePatch) Columns() map[string]string {
	out := map[string]string{}
	add := func(col string, v *string) {
		if v != nil {
			out[col] = *v
		}
	}
	add("full_name", p.FullName)
	add("department", p.JobTitle) // the User model's name for the job title
	add("phone", p.Phone)
	add("bio", p.Bio)
	add("timezone", p.Timezone)
	add("locale", p.Locale)
	add("date_format", p.DateFormat)
	add("theme_mode", p.ThemeMode)
	return out
}
