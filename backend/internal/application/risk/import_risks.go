// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package risk

import (
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"math"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
)

// ---------------------------------------------------------------------------
// CSV import of the risk register (#755).
//
// All-or-nothing, like bulk actions (D-036). Every row is validated before
// anything is written; one bad row means nothing is persisted and the caller
// gets the line and column of EVERY error, not just the first. Valid files are
// written inside ONE transaction, and each row goes through CreateRiskUseCase so
// an imported risk is born exactly like one typed into the form: same
// validation, same lifecycle entry, same owner fallback, same initial score.
//
// The scales are the product's: probability in [0,1], impact in [0,10]. The
// template this page used to hand out was on a 1–5 scale for both, and reading
// such a file as-is would produce plausible-looking but wrong scores. A file
// that looks like that scale is refused with a message saying so, never
// converted behind the user's back.
//
// This replaces a use case whose CSV parser returned zero rows and reported
// success, and that no route ever reached.
// ---------------------------------------------------------------------------

const (
	// MaxImportRows caps one file. Synchronous import inside one transaction is
	// adequate at this size; a larger register is several files.
	MaxImportRows = 1000
	// MaxImportBytes caps the upload. 1000 rows of long descriptions fit well
	// within it.
	MaxImportBytes = 2 << 20
)

// Import column names, lower-case. "name" is accepted as an alias of "title"
// because the register export and older files use it.
const (
	importColTitle       = "title"
	importColDescription = "description"
	importColProbability = "probability"
	importColImpact      = "impact"
	importColTags        = "tags"
	importColFrameworks  = "frameworks"
)

var importColumnAliases = map[string]string{
	"title":       importColTitle,
	"name":        importColTitle,
	"description": importColDescription,
	"probability": importColProbability,
	"impact":      importColImpact,
	"tags":        importColTags,
	"frameworks":  importColFrameworks,
	"framework":   importColFrameworks,
}

// ImportColumns is the accepted header, in template order.
var ImportColumns = []string{
	importColTitle, importColDescription, importColProbability,
	importColImpact, importColTags, importColFrameworks,
}

// RiskTxRunner runs fn inside one database transaction and hands it a risk
// repository bound to that transaction. An error from fn rolls everything back.
type RiskTxRunner func(ctx context.Context, fn func(repo domain.RiskRepository) error) error

// ImportCapacity reports how many more risks the tenant's plan allows.
// A negative value means unlimited.
type ImportCapacity func(ctx context.Context, tenantID uuid.UUID) (remaining int, err error)

// ImportRisksInput is one uploaded file.
type ImportRisksInput struct {
	CSV        []byte
	ImportedBy uuid.UUID
}

// ImportRowError locates one problem in the file. Line is the 1-based line in
// the file as a spreadsheet shows it (the header is line 1); 0 means the
// problem concerns the file as a whole. Column is the header name, empty when
// the problem is not about one cell.
//
// Code and Params are the contract: the page renders them in the reader's
// language. Message is the English rendering, for API callers and as the
// page's fallback when it does not know a code.
type ImportRowError struct {
	Line    int               `json:"line"`
	Column  string            `json:"column,omitempty"`
	Code    string            `json:"code"`
	Params  map[string]string `json:"params,omitempty"`
	Message string            `json:"message"`
}

// ImportRisksResult is what the caller is told. Under all-or-nothing, either
// Created is the number of data rows and Errors is empty, or Created is 0 and
// Rejected counts the rows that had at least one error.
type ImportRisksResult struct {
	Created  int              `json:"created"`
	Rejected int              `json:"rejected"`
	RiskIDs  []uuid.UUID      `json:"risk_ids"`
	Errors   []ImportRowError `json:"errors"`
	// Risks are the created entities, for the caller's post-commit work
	// (score events). Not serialised.
	Risks []*domain.Risk `json:"-"`
}

// ImportRejectedError carries a refused file's result. It matches
// domain.ErrValidation so generic handling still answers 4xx.
type ImportRejectedError struct {
	Result *ImportRisksResult
}

func (e *ImportRejectedError) Error() string {
	return fmt.Sprintf("import rejected: %d error(s), nothing was imported", len(e.Result.Errors))
}

func (e *ImportRejectedError) Unwrap() error { return domain.ErrValidation }

// ImportOverCapacityError means the file is valid but would take the tenant
// past its plan's risk limit. Nothing is written.
type ImportOverCapacityError struct {
	Requested int
	Remaining int
}

func (e *ImportOverCapacityError) Error() string {
	return fmt.Sprintf("import of %d risks exceeds the plan: %d more allowed", e.Requested, e.Remaining)
}

func (e *ImportOverCapacityError) Unwrap() error { return domain.ErrForbidden }

// ImportRisksUseCase imports a CSV file into the tenant's register.
type ImportRisksUseCase struct {
	inTx       RiskTxRunner
	capacity   ImportCapacity
	activation ActivationRecorder
}

// NewImportRisksUseCase builds the use case over a transaction runner.
func NewImportRisksUseCase(inTx RiskTxRunner) *ImportRisksUseCase {
	return &ImportRisksUseCase{inTx: inTx}
}

// WithCapacity attaches the plan-limit check. Nil-safe: without it there is no
// limit beyond MaxImportRows.
func (uc *ImportRisksUseCase) WithCapacity(c ImportCapacity) *ImportRisksUseCase {
	uc.capacity = c
	return uc
}

// WithActivation attaches the activation recorder, fed after commit only.
func (uc *ImportRisksUseCase) WithActivation(rec ActivationRecorder) *ImportRisksUseCase {
	uc.activation = rec
	return uc
}

// importRow is one parsed, validated data row.
type importRow struct {
	line  int
	input CreateRiskInput
}

// Execute validates the whole file, then creates every row in one transaction.
func (uc *ImportRisksUseCase) Execute(ctx context.Context, tenantID uuid.UUID, input ImportRisksInput) (*ImportRisksResult, error) {
	if tenantID == uuid.Nil {
		return nil, domain.NewUnauthorizedError("tenant is required")
	}
	if len(input.CSV) > MaxImportBytes {
		return nil, domain.NewValidationError(fmt.Sprintf("file is larger than %d MB", MaxImportBytes>>20))
	}

	rows, errs := parseImportCSV(input.CSV, input.ImportedBy)
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

	created := make([]*domain.Risk, 0, len(rows))
	err := uc.inTx(ctx, func(repo domain.RiskRepository) error {
		create := NewCreateRiskUseCase(repo)
		for _, row := range rows {
			r, err := create.Execute(ctx, tenantID, row.input)
			if err != nil {
				var appErr *domain.AppError
				if errors.As(err, &appErr) && errors.Is(err, domain.ErrValidation) {
					return &ImportRejectedError{Result: rejected([]ImportRowError{{Line: row.line, Code: "rejected_by_rules", Message: appErr.Message}})}
				}
				return err
			}
			created = append(created, r)
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

	result := &ImportRisksResult{
		Created: len(created),
		RiskIDs: make([]uuid.UUID, 0, len(created)),
		Errors:  []ImportRowError{},
		Risks:   created,
	}
	for _, r := range created {
		result.RiskIDs = append(result.RiskIDs, r.ID)
	}

	if uc.activation != nil && len(created) > 0 {
		uc.activation.Record(ctx, tenantID, string(domain.ActivationRiskCreated), map[string]interface{}{
			"risk_id": created[0].ID.String(),
			"source":  string(domain.SourceImport),
			"count":   len(created),
		})
	}
	return result, nil
}

// rejected builds the result of a refused file: nothing created, every
// offending line counted once.
func rejected(errs []ImportRowError) *ImportRisksResult {
	lines := map[int]bool{}
	for _, e := range errs {
		if e.Line > 1 {
			lines[e.Line] = true
		}
	}
	sort.SliceStable(errs, func(i, j int) bool { return errs[i].Line < errs[j].Line })
	return &ImportRisksResult{Created: 0, Rejected: len(lines), RiskIDs: []uuid.UUID{}, Errors: errs}
}

// parseImportCSV reads and validates the file. It returns either rows or
// errors, never both.
func parseImportCSV(data []byte, importedBy uuid.UUID) ([]importRow, []ImportRowError) {
	data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf")) // Excel's UTF-8 BOM
	if len(bytes.TrimSpace(data)) == 0 {
		return nil, []ImportRowError{{Line: 0, Code: "file_empty", Message: "the file is empty"}}
	}
	if !utf8.Valid(data) {
		return nil, []ImportRowError{{Line: 0, Code: "not_utf8", Message: "the file is not UTF-8 text; save it as \"CSV UTF-8\""}}
	}

	// French-locale spreadsheets export CSV with ";" and a decimal comma.
	firstLine, _, _ := bytes.Cut(data, []byte("\n"))
	semicolon := bytes.Count(firstLine, []byte(";")) > bytes.Count(firstLine, []byte(","))

	reader := csv.NewReader(bytes.NewReader(data))
	if semicolon {
		reader.Comma = ';'
	}
	reader.FieldsPerRecord = -1 // checked per row so the error carries a line
	reader.TrimLeadingSpace = true

	header, err := reader.Read()
	if err != nil {
		return nil, []ImportRowError{{Line: 1, Code: "header_unreadable", Params: map[string]string{"detail": err.Error()}, Message: fmt.Sprintf("the header cannot be read: %v", err)}}
	}

	var errs []ImportRowError
	cols := map[string]int{}
	names := make([]string, len(header))
	for i, h := range header {
		raw := strings.TrimSpace(h)
		canon, ok := importColumnAliases[strings.ToLower(raw)]
		if !ok {
			errs = append(errs, ImportRowError{Line: 1, Column: raw, Code: "unknown_column", Params: map[string]string{"column": raw, "accepted": strings.Join(ImportColumns, ", ")}, Message: fmt.Sprintf(
				"unknown column %q; accepted columns are %s", raw, strings.Join(ImportColumns, ", "))})
			continue
		}
		if _, dup := cols[canon]; dup {
			errs = append(errs, ImportRowError{Line: 1, Column: raw, Code: "duplicate_column", Params: map[string]string{"column": canon}, Message: fmt.Sprintf("column %q appears twice", canon)})
			continue
		}
		cols[canon] = i
		names[i] = canon
	}
	for _, required := range []string{importColTitle, importColProbability, importColImpact} {
		if _, ok := cols[required]; !ok {
			errs = append(errs, ImportRowError{Line: 1, Column: required, Code: "missing_column", Params: map[string]string{"column": required}, Message: fmt.Sprintf("required column %q is missing", required)})
		}
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
	number := func(s string) (float64, error) {
		if semicolon {
			s = strings.Replace(s, ",", ".", 1)
		}
		v, err := strconv.ParseFloat(s, 64)
		if err == nil && (math.IsNaN(v) || math.IsInf(v, 0)) {
			err = errors.New("not a finite number")
		}
		return v, err
	}

	var rows []importRow
	// legacy stays true while every row reads as the old 1–5 template: whole
	// numbers between 1 and 5 for both probability and impact.
	legacy := true
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
			errs = append(errs, ImportRowError{Line: line, Code: "line_unreadable", Params: map[string]string{"detail": err.Error()}, Message: fmt.Sprintf("the line cannot be read: %v", err)})
			legacy = false
			// A broken quote can swallow the rest of the file; stop here rather
			// than report a cascade of phantom errors.
			break
		}
		line, _ := reader.FieldPos(0)
		if isBlank(rec) {
			continue
		}
		dataRows++
		if dataRows > MaxImportRows {
			return nil, []ImportRowError{{Line: 0, Code: "too_many_rows", Params: map[string]string{"max": strconv.Itoa(MaxImportRows)}, Message: fmt.Sprintf("the file has more than %d rows; split it into several files", MaxImportRows)}}
		}
		if len(rec) > len(header) {
			errs = append(errs, ImportRowError{Line: line, Code: "cell_count", Params: map[string]string{"cells": strconv.Itoa(len(rec)), "header": strconv.Itoa(len(header))}, Message: fmt.Sprintf("the line has %d cells but the header has %d", len(rec), len(header))})
			continue
		}

		rowOK := true
		title := cell(rec, importColTitle)
		switch {
		case title == "":
			errs = append(errs, ImportRowError{Line: line, Column: importColTitle, Code: "required", Message: "title is required"})
			rowOK = false
		case utf8.RuneCountInString(title) > 255:
			errs = append(errs, ImportRowError{Line: line, Column: importColTitle, Code: "too_long", Params: map[string]string{"max": "255"}, Message: "title must be 255 characters or less"})
			rowOK = false
		}

		prob, perr := number(cell(rec, importColProbability))
		switch {
		case cell(rec, importColProbability) == "":
			errs = append(errs, ImportRowError{Line: line, Column: importColProbability, Code: "required", Message: "probability is required"})
			rowOK = false
		case perr != nil:
			errs = append(errs, ImportRowError{Line: line, Column: importColProbability, Code: "not_a_number", Params: map[string]string{"value": cell(rec, importColProbability)}, Message: fmt.Sprintf("%q is not a number", cell(rec, importColProbability))})
			rowOK = false
		case prob < 0 || prob > 1:
			errs = append(errs, ImportRowError{Line: line, Column: importColProbability, Code: "out_of_range", Params: map[string]string{"min": "0", "max": "1", "value": cell(rec, importColProbability)}, Message: fmt.Sprintf("probability must be between 0 and 1 (got %s)", cell(rec, importColProbability))})
			rowOK = false
		}

		imp, ierr := number(cell(rec, importColImpact))
		switch {
		case cell(rec, importColImpact) == "":
			errs = append(errs, ImportRowError{Line: line, Column: importColImpact, Code: "required", Message: "impact is required"})
			rowOK = false
		case ierr != nil:
			errs = append(errs, ImportRowError{Line: line, Column: importColImpact, Code: "not_a_number", Params: map[string]string{"value": cell(rec, importColImpact)}, Message: fmt.Sprintf("%q is not a number", cell(rec, importColImpact))})
			rowOK = false
		case imp < 0 || imp > 10:
			errs = append(errs, ImportRowError{Line: line, Column: importColImpact, Code: "out_of_range", Params: map[string]string{"min": "0", "max": "10", "value": cell(rec, importColImpact)}, Message: fmt.Sprintf("impact must be between 0 and 10 (got %s)", cell(rec, importColImpact))})
			rowOK = false
		}

		if perr != nil || ierr != nil || !onLegacyScale(prob) || !onLegacyScale(imp) {
			legacy = false
		}
		if !rowOK {
			continue
		}
		rows = append(rows, importRow{line: line, input: CreateRiskInput{
			Title:       title,
			Description: cell(rec, importColDescription),
			Probability: prob,
			Impact:      imp,
			Tags:        splitList(cell(rec, importColTags)),
			Frameworks:  splitList(cell(rec, importColFrameworks)),
			Source:      string(domain.SourceImport),
			CreatedBy:   importedBy,
		}})
	}

	if dataRows == 0 && len(errs) == 0 {
		return nil, []ImportRowError{{Line: 0, Code: "no_rows", Message: "the file has a header but no risk rows"}}
	}
	if legacy && dataRows > 0 {
		// Reported alone: the per-row range errors it also triggers would only
		// repeat the same cause once per line.
		return nil, []ImportRowError{{Line: 0, Column: importColProbability, Code: "legacy_scale", Message: "this file uses the old 1–5 scale for probability and impact; " +
			"OpenRisk expects probability between 0 and 1 and impact between 0 and 10. " +
			"Download the current template and convert the values (for example probability 3/5 → 0.6, impact 4/5 → 8)"}}
	}
	if len(errs) > 0 {
		return nil, errs
	}
	return rows, nil
}

func onLegacyScale(v float64) bool {
	return v >= 1 && v <= 5 && v == math.Trunc(v)
}

func isBlank(rec []string) bool {
	for _, v := range rec {
		if strings.TrimSpace(v) != "" {
			return false
		}
	}
	return true
}

// splitList reads a multi-value cell. "|" or ";" separate values; a "," does
// too when the cell holds neither, which is how most people type a list.
func splitList(s string) []string {
	if s == "" {
		return nil
	}
	sep := ","
	switch {
	case strings.Contains(s, "|"):
		sep = "|"
	case strings.Contains(s, ";"):
		sep = ";"
	}
	var out []string
	seen := map[string]bool{}
	for _, part := range strings.Split(s, sep) {
		p := strings.TrimSpace(part)
		if p != "" && !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	return out
}
