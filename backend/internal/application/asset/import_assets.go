// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package asset

import (
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
)

// ---------------------------------------------------------------------------
// CSV import of the asset inventory (#861).
//
// Same contract as the risk import (#755): every row is validated before
// anything is written, one bad row means nothing is persisted, and every error
// comes back with its line, column, a stable code the page translates, and an
// English message. A valid file is written in ONE transaction through
// CreateAssetUseCase, so an imported asset is born like one typed into the form.
//
// A name already in the tenant's inventory, or repeated in the file, is
// refused: the risk import links assets by name, and re-importing the same
// file must never double the inventory.
// ---------------------------------------------------------------------------

const (
	// MaxImportRows caps one file, as for risks.
	MaxImportRows = 1000
	// MaxImportBytes caps the upload, as for risks.
	MaxImportBytes = 2 << 20
	// maxImportNameLen bounds a name, as the form does.
	maxImportNameLen = 255
)

const (
	importColName        = "name"
	importColType        = "type"
	importColCriticality = "criticality"
	importColOwner       = "owner"
)

var importColumnAliases = map[string]string{
	"name":        importColName,
	"nom":         importColName,
	"type":        importColType,
	"criticality": importColCriticality,
	"criticité":   importColCriticality,
	"criticite":   importColCriticality,
	"owner":       importColOwner,
}

// ImportColumns is the accepted header, in template order.
var ImportColumns = []string{importColName, importColType, importColCriticality, importColOwner}

// importCriticalities maps accepted spellings, upper-cased, to the domain value.
// French spellings are accepted because French-locale teams write them.
var importCriticalities = map[string]domain.AssetCriticality{
	"LOW": domain.CriticalityLow, "FAIBLE": domain.CriticalityLow,
	"MEDIUM": domain.CriticalityMedium, "MOYENNE": domain.CriticalityMedium, "MOYEN": domain.CriticalityMedium,
	"HIGH": domain.CriticalityHigh, "ÉLEVÉE": domain.CriticalityHigh, "ELEVEE": domain.CriticalityHigh, "ÉLEVÉ": domain.CriticalityHigh, "ELEVE": domain.CriticalityHigh,
	"CRITICAL": domain.CriticalityCritical, "CRITIQUE": domain.CriticalityCritical,
}

// AssetTxRunner runs fn inside one database transaction with an asset
// repository bound to it. An error from fn rolls everything back.
type AssetTxRunner func(ctx context.Context, fn func(repo domain.AssetRepository) error) error

// ExistingAssetNames lists the names of the tenant's live assets, tenant-scoped.
type ExistingAssetNames func(ctx context.Context, tenantID uuid.UUID) ([]string, error)

// ImportCapacity reports how many more assets the tenant's plan allows.
// A negative value means unlimited.
type ImportCapacity func(ctx context.Context, tenantID uuid.UUID) (remaining int, err error)

// ImportAssetsInput is one uploaded file.
type ImportAssetsInput struct {
	CSV []byte
}

// ImportRowError locates one problem in the file; same wire shape as the risk
// import's. Line is 1-based with the header on line 1; 0 means the whole file.
type ImportRowError struct {
	Line    int               `json:"line"`
	Column  string            `json:"column,omitempty"`
	Code    string            `json:"code"`
	Params  map[string]string `json:"params,omitempty"`
	Message string            `json:"message"`
}

// ImportAssetsResult is what the caller is told.
type ImportAssetsResult struct {
	Created  int              `json:"created"`
	Rejected int              `json:"rejected"`
	AssetIDs []uuid.UUID      `json:"asset_ids"`
	Errors   []ImportRowError `json:"errors"`
}

// ImportRejectedError carries a refused file's result; it matches
// domain.ErrValidation.
type ImportRejectedError struct {
	Result *ImportAssetsResult
}

func (e *ImportRejectedError) Error() string {
	return fmt.Sprintf("import rejected: %d error(s), nothing was imported", len(e.Result.Errors))
}

func (e *ImportRejectedError) Unwrap() error { return domain.ErrValidation }

// ImportOverCapacityError means the file would take the tenant past its plan's
// asset limit. Nothing is written.
type ImportOverCapacityError struct {
	Requested int
	Remaining int
}

func (e *ImportOverCapacityError) Error() string {
	return fmt.Sprintf("import of %d assets exceeds the plan: %d more allowed", e.Requested, e.Remaining)
}

func (e *ImportOverCapacityError) Unwrap() error { return domain.ErrForbidden }

// ImportAssetsUseCase imports a CSV file into the tenant's inventory.
type ImportAssetsUseCase struct {
	inTx       AssetTxRunner
	existing   ExistingAssetNames
	capacity   ImportCapacity
	activation ActivationRecorder
}

// NewImportAssetsUseCase builds the use case. existing is required: without
// it a re-import would silently duplicate the inventory.
func NewImportAssetsUseCase(inTx AssetTxRunner, existing ExistingAssetNames) *ImportAssetsUseCase {
	return &ImportAssetsUseCase{inTx: inTx, existing: existing}
}

// WithCapacity attaches the plan-limit check.
func (uc *ImportAssetsUseCase) WithCapacity(c ImportCapacity) *ImportAssetsUseCase {
	uc.capacity = c
	return uc
}

// WithActivation attaches the activation recorder, fed after commit only.
func (uc *ImportAssetsUseCase) WithActivation(rec ActivationRecorder) *ImportAssetsUseCase {
	uc.activation = rec
	return uc
}

type importAssetRow struct {
	line  int
	input CreateAssetInput
}

// Execute validates the whole file, then creates every row in one transaction.
func (uc *ImportAssetsUseCase) Execute(ctx context.Context, tenantID uuid.UUID, input ImportAssetsInput) (*ImportAssetsResult, error) {
	if tenantID == uuid.Nil {
		return nil, domain.NewUnauthorizedError("tenant is required")
	}
	if len(input.CSV) > MaxImportBytes {
		return nil, domain.NewValidationError(fmt.Sprintf("file is larger than %d MB", MaxImportBytes>>20))
	}

	rows, errs := parseAssetCSV(input.CSV)

	// Names already in the inventory. Checked even when other rows failed, so
	// the user sees every problem in one pass.
	if len(rows) > 0 {
		names, err := uc.existing(ctx, tenantID)
		if err != nil {
			return nil, domain.NewInternalError(fmt.Sprintf("failed to list assets: %v", err))
		}
		taken := make(map[string]bool, len(names))
		for _, n := range names {
			taken[nameKey(n)] = true
		}
		for _, r := range rows {
			if taken[nameKey(r.input.Name)] {
				errs = append(errs, ImportRowError{Line: r.line, Column: importColName, Code: "asset_exists",
					Params:  map[string]string{"value": r.input.Name},
					Message: fmt.Sprintf("an asset named %q is already in the inventory", r.input.Name)})
			}
		}
	}
	if len(errs) > 0 {
		return nil, &ImportRejectedError{Result: rejected(errs)}
	}

	if uc.capacity != nil {
		remaining, err := uc.capacity(ctx, tenantID)
		// A counting error fails open, as the per-create middleware does.
		if err == nil && remaining >= 0 && len(rows) > remaining {
			return nil, &ImportOverCapacityError{Requested: len(rows), Remaining: remaining}
		}
	}

	created := make([]*domain.Asset, 0, len(rows))
	err := uc.inTx(ctx, func(repo domain.AssetRepository) error {
		create := NewCreateAssetUseCase(repo)
		for _, row := range rows {
			a, err := create.Execute(ctx, tenantID, row.input)
			if err != nil {
				var appErr *domain.AppError
				if errors.As(err, &appErr) && errors.Is(err, domain.ErrValidation) {
					return &ImportRejectedError{Result: rejected([]ImportRowError{{Line: row.line, Code: "rejected_by_rules", Message: appErr.Message}})}
				}
				return err
			}
			created = append(created, a)
		}
		return nil
	})
	if err != nil {
		var rej *ImportRejectedError
		if errors.As(err, &rej) {
			return nil, rej
		}
		return nil, domain.NewInternalError(fmt.Sprintf("import transaction failed: %v", err))
	}

	result := &ImportAssetsResult{
		Created:  len(created),
		AssetIDs: make([]uuid.UUID, 0, len(created)),
		Errors:   []ImportRowError{},
	}
	for _, a := range created {
		result.AssetIDs = append(result.AssetIDs, a.ID)
	}
	if uc.activation != nil && len(created) > 0 {
		uc.activation.Record(ctx, tenantID, string(domain.ActivationAssetConnected), map[string]interface{}{
			"asset_id": created[0].ID.String(),
			"source":   "IMPORT",
			"count":    len(created),
		})
	}
	return result, nil
}

func nameKey(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

// rejected builds the result of a refused file: nothing created, every
// offending line counted once.
func rejected(errs []ImportRowError) *ImportAssetsResult {
	lines := map[int]bool{}
	for _, e := range errs {
		if e.Line > 1 {
			lines[e.Line] = true
		}
	}
	sort.SliceStable(errs, func(i, j int) bool { return errs[i].Line < errs[j].Line })
	return &ImportAssetsResult{Created: 0, Rejected: len(lines), AssetIDs: []uuid.UUID{}, Errors: errs}
}

// parseAssetCSV reads and validates the file. With errors it may still return
// the rows it could read, so their names are checked against the inventory in
// the same pass; such a file is never written.
func parseAssetCSV(data []byte) ([]importAssetRow, []ImportRowError) {
	data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf")) // Excel's UTF-8 BOM
	if len(bytes.TrimSpace(data)) == 0 {
		return nil, []ImportRowError{{Code: "file_empty", Message: "the file is empty"}}
	}
	if !utf8.Valid(data) {
		return nil, []ImportRowError{{Code: "not_utf8", Message: "the file is not UTF-8 text; save it as \"CSV UTF-8\""}}
	}

	firstLine, _, _ := bytes.Cut(data, []byte("\n"))
	reader := csv.NewReader(bytes.NewReader(data))
	if bytes.Count(firstLine, []byte(";")) > bytes.Count(firstLine, []byte(",")) {
		reader.Comma = ';'
	}
	reader.FieldsPerRecord = -1
	reader.TrimLeadingSpace = true

	header, err := reader.Read()
	if err != nil {
		return nil, []ImportRowError{{Line: 1, Code: "header_unreadable", Params: map[string]string{"detail": err.Error()},
			Message: fmt.Sprintf("the header cannot be read: %v", err)}}
	}

	var errs []ImportRowError
	cols := map[string]int{}
	for i, h := range header {
		raw := strings.TrimSpace(h)
		canon, ok := importColumnAliases[strings.ToLower(raw)]
		if !ok {
			errs = append(errs, ImportRowError{Line: 1, Column: raw, Code: "unknown_column",
				Params:  map[string]string{"column": raw, "accepted": strings.Join(ImportColumns, ", ")},
				Message: fmt.Sprintf("unknown column %q; accepted columns are %s", raw, strings.Join(ImportColumns, ", "))})
			continue
		}
		if _, dup := cols[canon]; dup {
			errs = append(errs, ImportRowError{Line: 1, Column: raw, Code: "duplicate_column",
				Params: map[string]string{"column": canon}, Message: fmt.Sprintf("column %q appears twice", canon)})
			continue
		}
		cols[canon] = i
	}
	if _, ok := cols[importColName]; !ok {
		errs = append(errs, ImportRowError{Line: 1, Column: importColName, Code: "missing_column",
			Params: map[string]string{"column": importColName}, Message: `required column "name" is missing`})
	}
	if len(errs) > 0 {
		return nil, errs
	}

	cell := func(rec []string, col string) string {
		i, ok := cols[col]
		if !ok || i >= len(rec) {
			return ""
		}
		return strings.TrimSpace(rec[i])
	}

	var rows []importAssetRow
	firstLineOf := map[string]int{}
	dataRows := 0
	for {
		rec, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			line := 0
			var pe *csv.ParseError
			if errors.As(err, &pe) {
				line = pe.StartLine
			}
			errs = append(errs, ImportRowError{Line: line, Code: "line_unreadable", Params: map[string]string{"detail": err.Error()},
				Message: fmt.Sprintf("the line cannot be read: %v", err)})
			break // a broken quote can swallow the rest of the file
		}
		line, _ := reader.FieldPos(0)
		if isBlank(rec) {
			continue
		}
		dataRows++
		if dataRows > MaxImportRows {
			return nil, []ImportRowError{{Code: "too_many_rows", Params: map[string]string{"max": strconv.Itoa(MaxImportRows)},
				Message: fmt.Sprintf("the file has more than %d rows; split it into several files", MaxImportRows)}}
		}
		if len(rec) > len(header) {
			errs = append(errs, ImportRowError{Line: line, Code: "cell_count",
				Params:  map[string]string{"cells": strconv.Itoa(len(rec)), "header": strconv.Itoa(len(header))},
				Message: fmt.Sprintf("the line has %d cells but the header has %d", len(rec), len(header))})
			continue
		}

		rowOK := true
		name := cell(rec, importColName)
		switch {
		case name == "":
			errs = append(errs, ImportRowError{Line: line, Column: importColName, Code: "required", Message: "name is required"})
			rowOK = false
		case utf8.RuneCountInString(name) > maxImportNameLen:
			errs = append(errs, ImportRowError{Line: line, Column: importColName, Code: "too_long",
				Params: map[string]string{"max": strconv.Itoa(maxImportNameLen)}, Message: "name must be 255 characters or less"})
			rowOK = false
		default:
			if first, dup := firstLineOf[nameKey(name)]; dup {
				errs = append(errs, ImportRowError{Line: line, Column: importColName, Code: "duplicate_in_file",
					Params:  map[string]string{"value": name, "line": strconv.Itoa(first)},
					Message: fmt.Sprintf("%q already appears on line %d", name, first)})
				rowOK = false
			} else {
				firstLineOf[nameKey(name)] = line
			}
		}

		var criticality domain.AssetCriticality
		if raw := cell(rec, importColCriticality); raw != "" {
			c, ok := importCriticalities[strings.ToUpper(raw)]
			if !ok {
				errs = append(errs, ImportRowError{Line: line, Column: importColCriticality, Code: "invalid_criticality",
					Params:  map[string]string{"value": raw},
					Message: fmt.Sprintf("%q is not a criticality; use LOW, MEDIUM, HIGH or CRITICAL", raw)})
				rowOK = false
			}
			criticality = c
		}

		if !rowOK {
			continue
		}
		rows = append(rows, importAssetRow{line: line, input: CreateAssetInput{
			Name:        name,
			Type:        cell(rec, importColType),
			Criticality: criticality,
			Owner:       cell(rec, importColOwner),
			Source:      "IMPORT",
		}})
	}

	if dataRows == 0 && len(errs) == 0 {
		return nil, []ImportRowError{{Code: "no_rows", Message: "the file has a header but no asset rows"}}
	}
	return rows, errs
}

func isBlank(rec []string) bool {
	for _, v := range rec {
		if strings.TrimSpace(v) != "" {
			return false
		}
	}
	return true
}
