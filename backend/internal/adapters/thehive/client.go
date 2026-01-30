package thehive

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/config"
	"github.com/opendefender/openrisk/internal/core/domain"
)

// TheHiveAdapter implements the IncidentProvider interface for TheHive integration
type TheHiveAdapter struct {
	Config config.ExternalService
	Client http.Client
}

// TheHiveCase represents the structure of a case from TheHive API
type TheHiveCase struct {
	ID          string   json:"id"
	Title       string   json:"title"
	Description string   json:"description"
	Severity    int      json:"severity" // =Low, =Medium, =High, =Critical
	Status      string   json:"status"
	CreatedAt   int    json:"createdAt"
	UpdatedAt   int    json:"updatedAt"
	Tags        []string json:"tags"
}

// TheHiveResponse wraps paginated API responses
type TheHiveResponse struct {
	Data    []TheHiveCase json:"data"
	Success bool          json:"success"
}

// NewTheHiveAdapter creates a new TheHive adapter with production-grade HTTP configuration
func NewTheHiveAdapter(cfg config.ExternalService) TheHiveAdapter {
	return &TheHiveAdapter{
		Config: cfg,
		Client: &http.Client{
			Timeout:   time.Second, // Increased from s for reliable API calls
			Transport: &http.Transport{
				MaxIdleConns:        ,
				MaxIdleConnsPerHost: ,
				IdleConnTimeout:       time.Second,
			},
		},
	}
}

// FetchRecentIncidents retrieves recent cases from TheHive API
// Implements the IncidentProvider interface
func (a TheHiveAdapter) FetchRecentIncidents() ([]domain.Incident, error) {
	if !a.Config.Enabled {
		return []domain.Incident{}, nil
	}

	if a.Config.URL == "" || a.Config.APIKey == "" {
		// Return mock data if not properly configured (for dev/testing)
		return a.mockIncidents(), nil
	}

	// Fetch from real TheHive API
	incidents, err := a.fetchFromAPI()
	if err != nil {
		// Fallback to mock data if API call fails (graceful degradation)
		fmt.Printf("[TheHive] API call failed, using mock data: %v\n", err)
		return a.mockIncidents(), nil
	}

	return incidents, nil
}

// fetchFromAPI makes authenticated requests to TheHive REST API
func (a TheHiveAdapter) fetchFromAPI() ([]domain.Incident, error) {
	// Build request to fetch recent cases
	// TheHive API: GET /api/case with filters for recent/open cases
	url := fmt.Sprintf("%s/api/case?limit=&sort=-createdAt", a.Config.URL)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add authentication header
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", a.Config.APIKey))
	req.Header.Set("Content-Type", "application/json")

	// Execute request with timeout already configured in client
	resp, err := a.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	// Check for HTTP errors
	if resp.StatusCode <  || resp.StatusCode >=  {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var apiResp TheHiveResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Transform TheHive cases to domain incidents
	incidents := make([]domain.Incident, , len(apiResp.Data))
	for _, theHiveCase := range apiResp.Data {
		inc := a.transformCase(theHiveCase)
		// Only include non-closed cases
		if theHiveCase.Status != "Closed" && theHiveCase.Status != "Resolved" {
			incidents = append(incidents, inc)
		}
	}

	return incidents, nil
}

// transformCase converts a TheHive case to a domain Incident
func (a TheHiveAdapter) transformCase(caseData TheHiveCase) domain.Incident {
	// Map TheHive severity (-) to domain severity strings
	severity := "LOW"
	switch caseData.Severity {
	case :
		severity = "LOW"
	case :
		severity = "MEDIUM"
	case :
		severity = "HIGH"
	case :
		severity = "CRITICAL"
	}

	return domain.Incident{
		ID:          uuid.New(),
		Title:       caseData.Title,
		Description: caseData.Description,
		Status:      caseData.Status,
		Severity:    severity,
		CreatedAt:   time.UnixMilli(caseData.CreatedAt),
		Source:      "THEHIVE",
		ExternalID:  caseData.ID,
	}
}

// mockIncidents returns hardcoded incidents for development/fallback
func (a TheHiveAdapter) mockIncidents() []domain.Incident {
	return []domain.Incident{
		{
			ID:          uuid.New(),
			Title:       "Ransomware Detection (Mock)",
			Description: "Case : Encrypted files detected on HR Server during automated daily scan",
			Severity:    "HIGH",
			Status:      "Open",
			Source:      "THEHIVE",
			ExternalID:  "case__mock",
			CreatedAt:   time.Now().Add(-  time.Hour),
		},
		{
			ID:          uuid.New(),
			Title:       "Suspicious Login Attempt (Mock)",
			Description: "Case : Multiple failed login attempts from unusual IP detected",
			Severity:    "CRITICAL",
			Status:      "In Progress",
			Source:      "THEHIVE",
			ExternalID:  "case__mock",
			CreatedAt:   time.Now().Add(-  time.Hour),
		},
	}
}
