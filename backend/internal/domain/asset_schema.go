// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package domain

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"net"
	"net/mail"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

// AssetCategory is the typed category an asset belongs to. Unlike the free-text
// Asset.Type (kept for display and for the long tail of things that do not fit),
// the category is a CLOSED vocabulary: it selects which attribute schema governs
// the asset, so a Database is never asked for its "assigned user" and a Vendor is
// never asked for its "firmware version".
type AssetCategory string

const (
	CategoryServer      AssetCategory = "server"
	CategoryWorkstation AssetCategory = "workstation"
	CategoryApplication AssetCategory = "application"
	CategoryDatabase    AssetCategory = "database"
	CategoryNetwork     AssetCategory = "network"
	CategoryCloud       AssetCategory = "cloud"
	CategoryVendor      AssetCategory = "vendor"
	CategoryData        AssetCategory = "data_processing"
)

// AssetCategories is the ordered, complete list of supported categories. The
// order is the one the UI renders.
var AssetCategories = []AssetCategory{
	CategoryServer, CategoryWorkstation, CategoryApplication, CategoryDatabase,
	CategoryNetwork, CategoryCloud, CategoryVendor, CategoryData,
}

// ParseAssetCategory validates a category string. An empty value is NOT silently
// accepted: an asset with no category has no schema, and a typed inventory whose
// types are optional degrades back into the untyped one it replaces.
func ParseAssetCategory(s string) (AssetCategory, error) {
	c := AssetCategory(strings.ToLower(strings.TrimSpace(s)))
	for _, known := range AssetCategories {
		if c == known {
			return c, nil
		}
	}
	return "", NewValidationError(fmt.Sprintf("unknown asset category %q (expected one of: %s)", s, joinCategories()))
}

func joinCategories() string {
	out := make([]string, 0, len(AssetCategories))
	for _, c := range AssetCategories {
		out = append(out, string(c))
	}
	return strings.Join(out, ", ")
}

// AttributeType is the datatype of one attribute. It drives three things at
// once, which is why it is a closed set: which widget the generated form renders,
// which server-side validation runs, and how a search value is compared.
type AttributeType string

const (
	AttrString     AttributeType = "string"
	AttrText       AttributeType = "text" // multi-line
	AttrNumber     AttributeType = "number"
	AttrInteger    AttributeType = "integer"
	AttrBoolean    AttributeType = "boolean"
	AttrEnum       AttributeType = "enum"
	AttrMultiEnum  AttributeType = "multi_enum"
	AttrDate       AttributeType = "date" // YYYY-MM-DD
	AttrIP         AttributeType = "ip"
	AttrIPList     AttributeType = "ip_list"
	AttrHostname   AttributeType = "hostname"
	AttrCIDR       AttributeType = "cidr"
	AttrURL        AttributeType = "url"
	AttrEmail      AttributeType = "email"
	AttrStringList AttributeType = "string_list"
)

// IsValid reports whether t is a known attribute type.
func (t AttributeType) IsValid() bool {
	switch t {
	case AttrString, AttrText, AttrNumber, AttrInteger, AttrBoolean, AttrEnum,
		AttrMultiEnum, AttrDate, AttrIP, AttrIPList, AttrHostname, AttrCIDR,
		AttrURL, AttrEmail, AttrStringList:
		return true
	}
	return false
}

// IsList reports whether values of this type are arrays.
func (t AttributeType) IsList() bool {
	return t == AttrMultiEnum || t == AttrIPList || t == AttrStringList
}

// AttributeDef declares ONE attribute of a category's schema. The set of these
// for a category IS the JSON Schema the spec asks for — expressed in a shape the
// form generator, the validator and the search index can all consume without
// each re-implementing a JSON Schema dialect.
type AttributeDef struct {
	Key      string        `json:"key"`   // stable machine key, snake_case
	Label    string        `json:"label"` // FR label (product's primary locale)
	LabelEN  string        `json:"label_en,omitempty"`
	Type     AttributeType `json:"type"`
	Required bool          `json:"required,omitempty"`
	Enum     []string      `json:"enum,omitempty"` // allowed values for enum/multi_enum
	Group    string        `json:"group,omitempty"`
	Help     string        `json:"help,omitempty"`
	Min      *float64      `json:"min,omitempty"`
	Max      *float64      `json:"max,omitempty"`
	// Fingerprint marks an attribute as an identity signal used to correlate
	// external findings with this asset (see pkg/assetmatch). The value is one of
	// the FingerprintRole constants.
	Fingerprint FingerprintRole `json:"fingerprint,omitempty"`
}

// FingerprintRole labels an attribute as carrying one of the identity signals
// used to correlate a scanner/CTI finding back to an asset.
type FingerprintRole string

const (
	FingerprintNone     FingerprintRole = ""
	FingerprintHostname FingerprintRole = "hostname"
	FingerprintIP       FingerprintRole = "ip"
	FingerprintCloudID  FingerprintRole = "cloud_id"
	FingerprintCPE      FingerprintRole = "cpe"
)

var attributeKeyRe = regexp.MustCompile(`^[a-z][a-z0-9_]{0,48}[a-z0-9]$`)

// AttributeDefList is a JSONB-persisted slice of attribute definitions.
type AttributeDefList []AttributeDef

// Value implements driver.Valuer for JSONB persistence.
func (l AttributeDefList) Value() (driver.Value, error) {
	if l == nil {
		return "[]", nil
	}
	b, err := json.Marshal(l)
	if err != nil {
		return nil, err
	}
	return string(b), nil
}

// Scan implements sql.Scanner for JSONB persistence.
func (l *AttributeDefList) Scan(v any) error {
	if v == nil {
		*l = nil
		return nil
	}
	switch b := v.(type) {
	case []byte:
		return json.Unmarshal(b, l)
	case string:
		return json.Unmarshal([]byte(b), l)
	default:
		return fmt.Errorf("cannot scan %T into AttributeDefList", v)
	}
}

// AssetAttributes is the JSONB bag of an asset's typed attribute values. Keys
// are AttributeDef.Key; values are already coerced to their declared type by
// ValidateAttributes before they ever reach the database.
type AssetAttributes map[string]any

// Value implements driver.Valuer for JSONB persistence.
func (a AssetAttributes) Value() (driver.Value, error) {
	if a == nil {
		return "{}", nil
	}
	b, err := json.Marshal(a)
	if err != nil {
		return nil, err
	}
	return string(b), nil
}

// Scan implements sql.Scanner for JSONB persistence.
func (a *AssetAttributes) Scan(v any) error {
	if v == nil {
		*a = nil
		return nil
	}
	switch b := v.(type) {
	case []byte:
		return json.Unmarshal(b, a)
	case string:
		return json.Unmarshal([]byte(b), a)
	default:
		return fmt.Errorf("cannot scan %T into AssetAttributes", v)
	}
}

// AssetTypeSchema is a tenant's schema for one asset category. Exactly one row
// per (tenant, category): a tenant that never edits anything still has a row,
// seeded from DefaultAttributes, so "what does a Server look like here?" always
// has a single answer that lives in the database rather than in the client.
type AssetTypeSchema struct {
	ID       uuid.UUID     `gorm:"type:uuid;primaryKey" json:"id"`
	TenantID uuid.UUID     `gorm:"type:uuid;not null;uniqueIndex:ux_asset_schema_tenant_category" json:"tenant_id"`
	Category AssetCategory `gorm:"type:varchar(32);not null;uniqueIndex:ux_asset_schema_tenant_category" json:"category"`
	// Label is the tenant-facing name of the category ("Serveur", "Fournisseur").
	Label string `gorm:"size:128" json:"label"`
	// Attributes is the schema itself.
	Attributes AttributeDefList `gorm:"type:jsonb" json:"attributes"`
	// Customized records whether the tenant edited the shipped default. It is
	// what lets the UI offer "reset to default" honestly.
	Customized bool `gorm:"default:false" json:"customized"`
	// Version increments on every accepted edit, so an attribute bag can be read
	// against the schema generation that produced it.
	Version int `gorm:"default:1" json:"version"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TableName overrides the default GORM table name.
func (AssetTypeSchema) TableName() string { return "asset_type_schemas" }

// AssetTypeSchemaRepository is the port for reading and writing tenant schemas.
//
// ABSOLUTE RULE: every method filters by tenant_id.
type AssetTypeSchemaRepository interface {
	// ListByTenant returns every schema row a tenant has.
	ListByTenant(ctx context.Context, tenantID uuid.UUID) ([]AssetTypeSchema, error)
	// GetByCategory returns one schema, or (nil, nil) when the tenant has none yet.
	GetByCategory(ctx context.Context, tenantID uuid.UUID, cat AssetCategory) (*AssetTypeSchema, error)
	// Upsert writes a schema for (tenant, category).
	Upsert(ctx context.Context, s *AssetTypeSchema) error
	// Delete removes a tenant's customization for a category (reverting it to the
	// shipped default on the next read).
	Delete(ctx context.Context, tenantID uuid.UUID, cat AssetCategory) error
}

// ValidateSchema checks a tenant-authored schema before it is persisted. A
// schema is the contract every asset of that category is validated against, so a
// broken one would silently make an entire category unwritable.
func ValidateSchema(defs []AttributeDef) error {
	if len(defs) == 0 {
		return NewValidationError("a schema must declare at least one attribute")
	}
	if len(defs) > 60 {
		return NewValidationError("a schema may declare at most 60 attributes")
	}
	seen := map[string]bool{}
	for i, d := range defs {
		key := strings.TrimSpace(d.Key)
		if !attributeKeyRe.MatchString(key) {
			return NewValidationError(fmt.Sprintf("attribute #%d: key %q must be snake_case (a-z, 0-9, _), 2-50 chars", i+1, d.Key))
		}
		if seen[key] {
			return NewValidationError(fmt.Sprintf("attribute %q is declared twice", key))
		}
		seen[key] = true
		if strings.TrimSpace(d.Label) == "" {
			return NewValidationError(fmt.Sprintf("attribute %q: a label is required", key))
		}
		if !d.Type.IsValid() {
			return NewValidationError(fmt.Sprintf("attribute %q: unknown type %q", key, d.Type))
		}
		if d.Type == AttrEnum || d.Type == AttrMultiEnum {
			if len(d.Enum) == 0 {
				return NewValidationError(fmt.Sprintf("attribute %q: type %s requires a non-empty list of allowed values", key, d.Type))
			}
			vals := map[string]bool{}
			for _, v := range d.Enum {
				if strings.TrimSpace(v) == "" {
					return NewValidationError(fmt.Sprintf("attribute %q: allowed values cannot be blank", key))
				}
				if vals[v] {
					return NewValidationError(fmt.Sprintf("attribute %q: duplicate allowed value %q", key, v))
				}
				vals[v] = true
			}
		}
		if d.Min != nil && d.Max != nil && *d.Min > *d.Max {
			return NewValidationError(fmt.Sprintf("attribute %q: min (%v) is greater than max (%v)", key, *d.Min, *d.Max))
		}
	}
	return nil
}

// ValidateAttributes checks a submitted attribute bag against a schema and
// returns the COERCED bag to persist (numbers as float64, booleans as bool,
// lists as []string, everything trimmed).
//
// It is deliberately strict in both directions:
//   - a required attribute that is absent or blank is an error, named;
//   - an attribute the schema does not declare is an error, named — silently
//     dropping it would lose data the user typed, and silently keeping it would
//     make the schema a suggestion rather than a contract.
//
// This is the ONLY validation path. The client-side form generator renders the
// same schema, but the server never trusts that it did.
func ValidateAttributes(defs []AttributeDef, in map[string]any) (AssetAttributes, error) {
	byKey := make(map[string]AttributeDef, len(defs))
	for _, d := range defs {
		byKey[d.Key] = d
	}

	// Unknown keys first, reported together and in a stable order so the message
	// does not shuffle between identical requests.
	var unknown []string
	for k := range in {
		if _, ok := byKey[k]; !ok {
			unknown = append(unknown, k)
		}
	}
	if len(unknown) > 0 {
		sort.Strings(unknown)
		return nil, NewValidationError(fmt.Sprintf("unknown attribute(s) for this asset category: %s", strings.Join(unknown, ", ")))
	}

	out := AssetAttributes{}
	for _, d := range defs {
		raw, present := in[d.Key]
		if !present || isBlank(raw) {
			if d.Required {
				return nil, NewValidationError(fmt.Sprintf("%s is required", d.Label))
			}
			continue
		}
		v, err := coerceAttribute(d, raw)
		if err != nil {
			return nil, err
		}
		out[d.Key] = v
	}
	return out, nil
}

func isBlank(v any) bool {
	switch t := v.(type) {
	case nil:
		return true
	case string:
		return strings.TrimSpace(t) == ""
	case []any:
		return len(t) == 0
	case []string:
		return len(t) == 0
	}
	return false
}

// coerceAttribute validates one value and returns its canonical Go form.
func coerceAttribute(d AttributeDef, raw any) (any, error) {
	fail := func(what string) error {
		return NewValidationError(fmt.Sprintf("%s: %s", d.Label, what))
	}

	if d.Type.IsList() {
		items, err := toStringSlice(raw)
		if err != nil {
			return nil, fail("expected a list of values")
		}
		out := make([]string, 0, len(items))
		for _, it := range items {
			it = strings.TrimSpace(it)
			if it == "" {
				continue
			}
			switch d.Type {
			case AttrMultiEnum:
				if !containsString(d.Enum, it) {
					return nil, fail(fmt.Sprintf("%q is not an allowed value (expected one of: %s)", it, strings.Join(d.Enum, ", ")))
				}
			case AttrIPList:
				if net.ParseIP(it) == nil {
					return nil, fail(fmt.Sprintf("%q is not a valid IP address", it))
				}
			}
			out = append(out, it)
		}
		if len(out) == 0 && d.Required {
			return nil, fail("is required")
		}
		return out, nil
	}

	switch d.Type {
	case AttrString, AttrText:
		s := strings.TrimSpace(fmt.Sprintf("%v", raw))
		if d.Max != nil && float64(len(s)) > *d.Max {
			return nil, fail(fmt.Sprintf("must be at most %d characters", int(*d.Max)))
		}
		return s, nil

	case AttrNumber, AttrInteger:
		f, err := toFloat(raw)
		if err != nil {
			return nil, fail("expected a number")
		}
		if d.Type == AttrInteger && f != float64(int64(f)) {
			return nil, fail("expected a whole number")
		}
		if d.Min != nil && f < *d.Min {
			return nil, fail(fmt.Sprintf("must be at least %v", *d.Min))
		}
		if d.Max != nil && f > *d.Max {
			return nil, fail(fmt.Sprintf("must be at most %v", *d.Max))
		}
		return f, nil

	case AttrBoolean:
		switch t := raw.(type) {
		case bool:
			return t, nil
		case string:
			b, err := strconv.ParseBool(strings.TrimSpace(t))
			if err != nil {
				return nil, fail("expected true or false")
			}
			return b, nil
		default:
			return nil, fail("expected true or false")
		}

	case AttrEnum:
		s := strings.TrimSpace(fmt.Sprintf("%v", raw))
		if !containsString(d.Enum, s) {
			return nil, fail(fmt.Sprintf("%q is not an allowed value (expected one of: %s)", s, strings.Join(d.Enum, ", ")))
		}
		return s, nil

	case AttrDate:
		s := strings.TrimSpace(fmt.Sprintf("%v", raw))
		if len(s) > 10 {
			s = s[:10] // tolerate a full RFC3339 timestamp from a date picker
		}
		if _, err := time.Parse("2006-01-02", s); err != nil {
			return nil, fail("expected a date in YYYY-MM-DD format")
		}
		return s, nil

	case AttrIP:
		s := strings.TrimSpace(fmt.Sprintf("%v", raw))
		if net.ParseIP(s) == nil {
			return nil, fail(fmt.Sprintf("%q is not a valid IP address", s))
		}
		return s, nil

	case AttrCIDR:
		s := strings.TrimSpace(fmt.Sprintf("%v", raw))
		if _, _, err := net.ParseCIDR(s); err != nil {
			return nil, fail(fmt.Sprintf("%q is not a valid CIDR range", s))
		}
		return s, nil

	case AttrHostname:
		s := strings.TrimSpace(fmt.Sprintf("%v", raw))
		if !isHostname(s) {
			return nil, fail(fmt.Sprintf("%q is not a valid hostname", s))
		}
		return s, nil

	case AttrURL:
		s := strings.TrimSpace(fmt.Sprintf("%v", raw))
		u, err := url.Parse(s)
		if err != nil || u.Scheme == "" || u.Host == "" {
			return nil, fail("expected a full URL (https://…)")
		}
		return s, nil

	case AttrEmail:
		s := strings.TrimSpace(fmt.Sprintf("%v", raw))
		if _, err := mail.ParseAddress(s); err != nil {
			return nil, fail(fmt.Sprintf("%q is not a valid email address", s))
		}
		return s, nil
	}
	return nil, fail(fmt.Sprintf("unsupported attribute type %q", d.Type))
}

var hostnameRe = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9\-_]{0,61}[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9\-_]{0,61}[a-zA-Z0-9])?)*$`)

func isHostname(s string) bool {
	return len(s) > 0 && len(s) <= 253 && hostnameRe.MatchString(s)
}

func containsString(list []string, v string) bool {
	for _, it := range list {
		if it == v {
			return true
		}
	}
	return false
}

func toFloat(raw any) (float64, error) {
	switch t := raw.(type) {
	case float64:
		return t, nil
	case float32:
		return float64(t), nil
	case int:
		return float64(t), nil
	case int64:
		return float64(t), nil
	case json.Number:
		return t.Float64()
	case string:
		return strconv.ParseFloat(strings.TrimSpace(t), 64)
	}
	return 0, fmt.Errorf("not a number")
}

func toStringSlice(raw any) ([]string, error) {
	switch t := raw.(type) {
	case []string:
		return t, nil
	case []any:
		out := make([]string, 0, len(t))
		for _, v := range t {
			out = append(out, strings.TrimSpace(fmt.Sprintf("%v", v)))
		}
		return out, nil
	case string:
		// A comma-separated string is what a plain <input> yields; accept it.
		parts := strings.Split(t, ",")
		out := make([]string, 0, len(parts))
		for _, p := range parts {
			out = append(out, strings.TrimSpace(p))
		}
		return out, nil
	}
	return nil, fmt.Errorf("not a list")
}

// attributeValuesAsStrings flattens one stored attribute value to a string slice.
func attributeValuesAsStrings(v any) []string {
	switch t := v.(type) {
	case []string:
		return t
	case []any:
		out := make([]string, 0, len(t))
		for _, it := range t {
			out = append(out, strings.TrimSpace(fmt.Sprintf("%v", it)))
		}
		return out
	case nil:
		return nil
	default:
		s := strings.TrimSpace(fmt.Sprintf("%v", t))
		if s == "" {
			return nil
		}
		return []string{s}
	}
}

// hostsFromURLs reduces URLs to their hostnames, so an application's URL can
// serve as a hostname fingerprint without the tenant duplicating the value.
func hostsFromURLs(vals []string) []string {
	out := make([]string, 0, len(vals))
	for _, v := range vals {
		if u, err := url.Parse(v); err == nil && u.Hostname() != "" {
			out = append(out, u.Hostname())
			continue
		}
		out = append(out, v)
	}
	return out
}

// AttributeSearchTerm is one parsed `attr.<key>=<value>` search filter.
type AttributeSearchTerm struct {
	Key   string
	Value string
}

// MatchesAttributes reports whether an attribute bag satisfies every search
// term. Comparison is case-insensitive substring for text, exact for booleans
// and enums, and "contains" for lists — the semantics a user expects when they
// type `environment=prod` in the inventory search box.
func MatchesAttributes(attrs AssetAttributes, terms []AttributeSearchTerm) bool {
	for _, t := range terms {
		v, ok := attrs[t.Key]
		if !ok {
			return false
		}
		if !attributeValueMatches(v, t.Value) {
			return false
		}
	}
	return true
}

func attributeValueMatches(v any, want string) bool {
	want = strings.ToLower(strings.TrimSpace(want))
	switch t := v.(type) {
	case []string:
		for _, it := range t {
			if strings.ToLower(it) == want {
				return true
			}
		}
		return false
	case []any:
		for _, it := range t {
			if strings.ToLower(fmt.Sprintf("%v", it)) == want {
				return true
			}
		}
		return false
	case bool:
		return strconv.FormatBool(t) == want
	case float64:
		return strings.EqualFold(strconv.FormatFloat(t, 'f', -1, 64), want)
	default:
		return strings.Contains(strings.ToLower(fmt.Sprintf("%v", t)), want)
	}
}
