// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package domain

import (
	"net/url"
	"strings"
	"time"
	// The IANA database is embedded so a time zone that validates on a
	// developer's machine also validates in a minimal container image.
	_ "time/tzdata"
	"unicode/utf8"
)

// Keys of the organization profile inside organizations.settings. timezone was
// already read from there; the others join it so no column is added.
const (
	OrgSettingTimezone      = "timezone"
	OrgSettingWebsite       = "website"
	OrgSettingDescription   = "description"
	OrgSettingDefaultLocale = "default_locale"
	OrgSettingDateFormat    = "date_format"
)

// DateFormat is how a date is written for a person or an organization.
type DateFormat string

const (
	DateFormatDMY DateFormat = "DD/MM/YYYY"
	DateFormatMDY DateFormat = "MM/DD/YYYY"
	DateFormatISO DateFormat = "YYYY-MM-DD"
)

// IsDateFormat reports whether f is one of the supported formats.
func IsDateFormat(f string) bool {
	switch DateFormat(f) {
	case DateFormatDMY, DateFormatMDY, DateFormatISO:
		return true
	}
	return false
}

// enabledLocales mirrors the `enabled: true` entries of the frontend locale
// registry (frontend/src/i18n/locales.ts). A locale is accepted here only once
// its catalogue ships there.
var enabledLocales = map[string]struct{}{"fr": {}, "en": {}}

// IsEnabledLocale reports whether code is a locale the product can render.
func IsEnabledLocale(code string) bool {
	_, ok := enabledLocales[code]
	return ok
}

// IsTimezone reports whether tz is a valid IANA zone name. "Local" is refused:
// it names the server's zone, which is nobody's preference.
func IsTimezone(tz string) bool {
	if tz == "" || tz == "Local" {
		return false
	}
	_, err := time.LoadLocation(tz)
	return err == nil
}

// IsOrgSize reports whether s is one of the organization size buckets.
func IsOrgSize(s string) bool {
	switch OrgSize(s) {
	case Size1to50, Size51to200, Size201to1000, Size1000Plus:
		return true
	}
	return false
}

// OrganizationProfilePatch is a partial update of the organization profile.
// A nil field is left unchanged. An empty string clears an optional field;
// Name cannot be cleared.
type OrganizationProfilePatch struct {
	Name          *string `json:"name,omitempty"`
	Industry      *string `json:"industry,omitempty"`
	Size          *string `json:"size,omitempty"`
	Website       *string `json:"website,omitempty"`
	Description   *string `json:"description,omitempty"`
	Timezone      *string `json:"timezone,omitempty"`
	DefaultLocale *string `json:"default_locale,omitempty"`
	DateFormat    *string `json:"date_format,omitempty"`
}

// Normalize trims every provided field and validates it. It returns the first
// violation as a validation error naming the field.
func (p *OrganizationProfilePatch) Normalize() error {
	trim := func(s *string) {
		if s != nil {
			*s = strings.TrimSpace(*s)
		}
	}
	for _, f := range []*string{p.Name, p.Industry, p.Size, p.Website, p.Description, p.Timezone, p.DefaultLocale, p.DateFormat} {
		trim(f)
	}

	if p.Name != nil {
		if n := utf8.RuneCountInString(*p.Name); n < 2 || n > 120 {
			return NewValidationError("name: must be between 2 and 120 characters")
		}
	}
	if p.Industry != nil && utf8.RuneCountInString(*p.Industry) > 80 {
		return NewValidationError("industry: must be at most 80 characters")
	}
	if p.Size != nil && *p.Size != "" && !IsOrgSize(*p.Size) {
		return NewValidationError("size: must be one of 1-50, 51-200, 201-1000, 1000+")
	}
	if p.Website != nil && *p.Website != "" && !isHTTPSURL(*p.Website) {
		return NewValidationError("website: must be an absolute https:// URL of at most 255 characters")
	}
	if p.Description != nil && utf8.RuneCountInString(*p.Description) > 500 {
		return NewValidationError("description: must be at most 500 characters")
	}
	if p.Timezone != nil && *p.Timezone != "" && !IsTimezone(*p.Timezone) {
		return NewValidationError("timezone: unknown IANA time zone")
	}
	if p.DefaultLocale != nil && *p.DefaultLocale != "" && !IsEnabledLocale(*p.DefaultLocale) {
		return NewValidationError("default_locale: unsupported locale")
	}
	if p.DateFormat != nil && *p.DateFormat != "" && !IsDateFormat(*p.DateFormat) {
		return NewValidationError("date_format: must be one of DD/MM/YYYY, MM/DD/YYYY, YYYY-MM-DD")
	}
	return nil
}

// IsEmpty reports whether the patch changes nothing.
func (p *OrganizationProfilePatch) IsEmpty() bool {
	return p.Name == nil && p.Industry == nil && p.Size == nil && p.Website == nil &&
		p.Description == nil && p.Timezone == nil && p.DefaultLocale == nil && p.DateFormat == nil
}

func isHTTPSURL(raw string) bool {
	if len(raw) > 255 {
		return false
	}
	u, err := url.Parse(raw)
	return err == nil && u.Scheme == "https" && u.Host != "" && u.User == nil
}
