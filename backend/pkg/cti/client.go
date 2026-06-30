// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package cti

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/lib/pq"
)

// ====================================================================
// HTTP Client Abstraction (for testing)
// ====================================================================

// HTTPDoer defines a minimal HTTP client interface for testability.
type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

// ====================================================================
// External Client (NVD + CISA KEV)
// ====================================================================

// ExternalClient fetches CTI feeds from NVD and CISA KEV.
// All requests: 30s timeout, 3 retries with exponential backoff (1s → 3s → 9s).
type ExternalClient struct {
	httpClient HTTPDoer
	maxRetries int
	nvdAPIKey  string // Optional NVD API key for higher rate limits
}

// NewExternalClient creates a new ExternalClient.
// If httpClient is nil, a default http.Client with 30s timeout is used.
func NewExternalClient(httpClient HTTPDoer, nvdAPIKey string) *ExternalClient {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	return &ExternalClient{
		httpClient: httpClient,
		maxRetries: 3,
		nvdAPIKey:  nvdAPIKey,
	}
}

// ====================================================================
// NVD API v2.0
// ====================================================================

// NVD API response structures (minimal, only what we need)
type nvdResponse struct {
	ResultsPerPage  int             `json:"resultsPerPage"`
	StartIndex      int             `json:"startIndex"`
	TotalResults    int             `json:"totalResults"`
	Vulnerabilities []nvdVulnEntry  `json:"vulnerabilities"`
}

type nvdVulnEntry struct {
	CVE nvdCVE `json:"cve"`
}

type nvdCVE struct {
	ID               string          `json:"id"`
	Published        string          `json:"published"`
	LastModified     string          `json:"lastModified"`
	Descriptions     []nvdLangString `json:"descriptions"`
	Metrics          nvdMetrics      `json:"metrics"`
	Configurations   []nvdConfig     `json:"configurations"`
	References       []nvdReference  `json:"references"`
}

type nvdLangString struct {
	Lang  string `json:"lang"`
	Value string `json:"value"`
}

type nvdMetrics struct {
	CvssMetricV31 []nvdCVSSMetric `json:"cvssMetricV31"`
	CvssMetricV30 []nvdCVSSMetric `json:"cvssMetricV30"`
}

type nvdCVSSMetric struct {
	CVSSData nvdCVSSData `json:"cvssData"`
}

type nvdCVSSData struct {
	BaseScore    float64 `json:"baseScore"`
	BaseSeverity string  `json:"baseSeverity"`
}

type nvdConfig struct {
	Nodes []nvdNode `json:"nodes"`
}

type nvdNode struct {
	CPEMatch []nvdCPEMatch `json:"cpeMatch"`
}

type nvdCPEMatch struct {
	Vulnerable bool   `json:"vulnerable"`
	Criteria   string `json:"criteria"`
}

type nvdReference struct {
	URL    string   `json:"url"`
	Source string   `json:"source"`
	Tags   []string `json:"tags"`
}

// FetchNVDCVEs fetches CVEs from NVD API v2.0, filtered by date range.
// Fetches CRITICAL and HIGH severity CVEs with resultsPerPage=2000.
func (c *ExternalClient) FetchNVDCVEs(ctx context.Context, pubStartDate, pubEndDate string) ([]CTIVulnerability, error) {
	const baseURL = "https://services.nvd.nist.gov/rest/json/cves/2.0"

	params := url.Values{}
	params.Set("resultsPerPage", "2000")
	if pubStartDate != "" {
		params.Set("pubStartDate", pubStartDate)
	}
	if pubEndDate != "" {
		params.Set("pubEndDate", pubEndDate)
	}

	fullURL := baseURL + "?" + params.Encode()

	data, err := c.fetchWithRetry(ctx, fullURL, c.nvdHeaders())
	if err != nil {
		return nil, fmt.Errorf("NVD fetch failed: %w", err)
	}

	var resp nvdResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("NVD parse failed: %w", err)
	}

	vulns := make([]CTIVulnerability, 0, len(resp.Vulnerabilities))
	for _, entry := range resp.Vulnerabilities {
		vuln := c.nvdEntryToVuln(entry)
		// Only keep HIGH and CRITICAL
		if vuln.Severity == "CRITICAL" || vuln.Severity == "HIGH" {
			vulns = append(vulns, vuln)
		}
	}

	return vulns, nil
}

// nvdEntryToVuln converts an NVD API entry to our domain model.
func (c *ExternalClient) nvdEntryToVuln(entry nvdVulnEntry) CTIVulnerability {
	cve := entry.CVE

	// Extract English description
	description := ""
	for _, desc := range cve.Descriptions {
		if desc.Lang == "en" {
			description = desc.Value
			break
		}
	}

	// Extract CVSS v3.1 score (fallback to v3.0)
	var cvssScore float64
	var severity string
	if len(cve.Metrics.CvssMetricV31) > 0 {
		cvssScore = cve.Metrics.CvssMetricV31[0].CVSSData.BaseScore
		severity = cve.Metrics.CvssMetricV31[0].CVSSData.BaseSeverity
	} else if len(cve.Metrics.CvssMetricV30) > 0 {
		cvssScore = cve.Metrics.CvssMetricV30[0].CVSSData.BaseScore
		severity = cve.Metrics.CvssMetricV30[0].CVSSData.BaseSeverity
	}

	// Extract CPEs
	var cpes []string
	for _, config := range cve.Configurations {
		for _, node := range config.Nodes {
			for _, match := range node.CPEMatch {
				if match.Vulnerable {
					cpes = append(cpes, match.Criteria)
				}
			}
		}
	}

	// Extract references as JSON
	refsJSON, _ := json.Marshal(cve.References)

	// Parse dates
	publishedAt, _ := time.Parse("2006-01-02T15:04:05.000", cve.Published)
	lastModified, _ := time.Parse("2006-01-02T15:04:05.000", cve.LastModified)

	now := time.Now().UTC()
	return CTIVulnerability{
		CVEID:         cve.ID,
		CVSSV3:        cvssScore,
		Severity:      severity,
		Description:   description,
		PublishedAt:   publishedAt,
		AffectedCPE:   pq.StringArray(cpes),
		References:    refsJSON,
		LastUpdatedAt: lastModified,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
}

func (c *ExternalClient) nvdHeaders() map[string]string {
	headers := map[string]string{
		"Accept": "application/json",
	}
	if c.nvdAPIKey != "" {
		headers["apiKey"] = c.nvdAPIKey
	}
	return headers
}

// ====================================================================
// CISA KEV (Known Exploited Vulnerabilities)
// ====================================================================

// CISA KEV response structures
type cisaKEVResponse struct {
	Title           string          `json:"title"`
	CatalogVersion  string          `json:"catalogVersion"`
	DateReleased    string          `json:"dateReleased"`
	Count           int             `json:"count"`
	Vulnerabilities []cisaKEVEntry  `json:"vulnerabilities"`
}

type cisaKEVEntry struct {
	CVEID              string `json:"cveID"`
	VendorProject      string `json:"vendorProject"`
	Product            string `json:"product"`
	VulnerabilityName  string `json:"vulnerabilityName"`
	DateAdded          string `json:"dateAdded"`
	ShortDescription   string `json:"shortDescription"`
	RequiredAction     string `json:"requiredAction"`
	DueDate            string `json:"dueDate"`
	KnownRansomware    string `json:"knownRansomwareCampaignUse"`
	Notes              string `json:"notes"`
}

// FetchCISAKEV fetches the CISA Known Exploited Vulnerabilities catalog.
// All returned CVEs are marked cisa_known=true with maximum criticality.
func (c *ExternalClient) FetchCISAKEV(ctx context.Context) ([]CTIVulnerability, error) {
	const kevURL = "https://www.cisa.gov/sites/default/files/feeds/known_exploited_vulnerabilities.json"

	data, err := c.fetchWithRetry(ctx, kevURL, nil)
	if err != nil {
		return nil, fmt.Errorf("CISA KEV fetch failed: %w", err)
	}

	var resp cisaKEVResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("CISA KEV parse failed: %w", err)
	}

	now := time.Now().UTC()
	vulns := make([]CTIVulnerability, 0, len(resp.Vulnerabilities))
	for _, entry := range resp.Vulnerabilities {
		vuln := c.cisaEntryToVuln(entry, now)
		vulns = append(vulns, vuln)
	}

	return vulns, nil
}

// cisaEntryToVuln converts a CISA KEV entry to our domain model.
// RULE: All CISA KEV → cisa_known = true, severity = CRITICAL.
func (c *ExternalClient) cisaEntryToVuln(entry cisaKEVEntry, now time.Time) CTIVulnerability {
	var dueDate *time.Time
	if entry.DueDate != "" {
		if parsed, err := time.Parse("2006-01-02", entry.DueDate); err == nil {
			dueDate = &parsed
		}
	}

	publishedAt := now
	if entry.DateAdded != "" {
		if parsed, err := time.Parse("2006-01-02", entry.DateAdded); err == nil {
			publishedAt = parsed
		}
	}

	return CTIVulnerability{
		CVEID:         entry.CVEID,
		Severity:      "CRITICAL", // RULE: All CISA KEV = maximum criticality
		CISAKnown:     true,
		CISADueDate:   dueDate,
		Description:   entry.ShortDescription,
		Remediation:   entry.RequiredAction,
		PublishedAt:   publishedAt,
		LastUpdatedAt: now,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
}

// ====================================================================
// HTTP Fetch with Retry + Exponential Backoff
// ====================================================================

// fetchWithRetry performs an HTTP GET with 3 retries and exponential backoff (1s → 3s → 9s).
func (c *ExternalClient) fetchWithRetry(ctx context.Context, rawURL string, headers map[string]string) ([]byte, error) {
	var lastErr error
	backoff := time.Second // 1s → 3s → 9s

	for attempt := 0; attempt < c.maxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
			backoff *= 3
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		for k, v := range headers {
			req.Header.Set(k, v)
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = err
			continue
		}

		body, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()

		if readErr != nil {
			lastErr = fmt.Errorf("failed to read response: %w", readErr)
			continue
		}

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return body, nil
		}

		lastErr = fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body[:min(200, len(body))]))
	}

	return nil, fmt.Errorf("fetch failed after %d retries: %w", c.maxRetries, lastErr)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
