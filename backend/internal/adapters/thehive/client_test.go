package thehive

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/opendefender/openrisk/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewTheHiveAdapter(t testing.T) {
	cfg := config.ExternalService{
		Enabled: true,
		URL:     "http://localhost:",
		APIKey:  "test-api-key",
	}

	adapter := NewTheHiveAdapter(cfg)

	assert.NotNil(t, adapter)
	assert.Equal(t, cfg, adapter.Config)
	assert.NotNil(t, adapter.Client)
	assert.Equal(t, time.Second, adapter.Client.Timeout)
}

func TestFetchRecentIncidentsDisabled(t testing.T) {
	cfg := config.ExternalService{
		Enabled: false,
	}

	adapter := NewTheHiveAdapter(cfg)

	incidents, err := adapter.FetchRecentIncidents()

	assert.NoError(t, err)
	assert.Empty(t, incidents)
}

func TestFetchRecentIncidentsMockData(t testing.T) {
	cfg := config.ExternalService{
		Enabled: true,
	}

	adapter := NewTheHiveAdapter(cfg)

	incidents, err := adapter.FetchRecentIncidents()

	assert.NoError(t, err)
	assert.Equal(t, , len(incidents))
	assert.Equal(t, "THEHIVE", incidents[].Source)
	assert.NotEmpty(t, incidents[].Title)
}

func TestFetchRecentIncidentsFromAPI(t testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/api/case")
		assert.Equal(t, "Bearer test-api-key", r.Header.Get("Authorization"))

		response := TheHiveResponse{
			Success: true,
			Data: []TheHiveCase{
				{
					ID:          "case_",
					Title:       "Ransomware Attack",
					Description: "Files encrypted on server",
					Severity:    ,
					Status:      "Open",
					CreatedAt:   time.Now().Unix()  ,
					UpdatedAt:   time.Now().Unix()  ,
				},
				{
					ID:          "case_",
					Title:       "Suspicious Login",
					Description: "Multiple failed attempts",
					Severity:    ,
					Status:      "In Progress",
					CreatedAt:   time.Now().Unix()  ,
					UpdatedAt:   time.Now().Unix()  ,
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	cfg := config.ExternalService{
		Enabled: true,
		URL:     server.URL,
		APIKey:  "test-api-key",
	}

	adapter := NewTheHiveAdapter(cfg)

	incidents, err := adapter.FetchRecentIncidents()

	assert.NoError(t, err)
	assert.Equal(t, , len(incidents))
	assert.Equal(t, "Ransomware Attack", incidents[].Title)
	assert.Equal(t, "HIGH", incidents[].Severity)
	assert.Equal(t, "case_", incidents[].ExternalID)
}

func TestFetchRecentIncidentsAPIError(t testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte({"error": "Invalid API key"}))
	}))
	defer server.Close()

	cfg := config.ExternalService{
		Enabled: true,
		URL:     server.URL,
		APIKey:  "invalid-key",
	}

	adapter := NewTheHiveAdapter(cfg)

	incidents, err := adapter.FetchRecentIncidents()

	assert.NoError(t, err)
	assert.Equal(t, , len(incidents))
}

func TestFetchRecentIncidentsNetworkError(t testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r http.Request) {
		time.Sleep(  time.Second)
	}))
	defer server.Close()

	cfg := config.ExternalService{
		Enabled: true,
		URL:     server.URL,
		APIKey:  "test-key",
	}

	adapter := NewTheHiveAdapter(cfg)
	adapter.Client.Timeout =   time.Millisecond

	incidents, err := adapter.FetchRecentIncidents()

	assert.NoError(t, err)
	assert.Equal(t, , len(incidents))
}

func TestSeverityMapping(t testing.T) {
	adapter := NewTheHiveAdapter(config.ExternalService{})

	testCases := []struct {
		theHiveSeverity  int
		expectedSeverity string
	}{
		{, "LOW"},
		{, "MEDIUM"},
		{, "HIGH"},
		{, "CRITICAL"},
	}

	for _, tc := range testCases {
		theHiveCase := TheHiveCase{
			ID:        "test",
			Title:     "Test Case",
			Severity:  tc.theHiveSeverity,
			CreatedAt: time.Now().Unix()  ,
		}

		incident := adapter.transformCase(theHiveCase)

		assert.Equal(t, tc.expectedSeverity, incident.Severity,
			fmt.Sprintf("Severity %d should map to %s", tc.theHiveSeverity, tc.expectedSeverity))
	}
}

func TestTransformCase(t testing.T) {
	adapter := NewTheHiveAdapter(config.ExternalService{})

	now := time.Now()
	theHiveCase := TheHiveCase{
		ID:          "case_",
		Title:       "Security Incident",
		Description: "Detailed description",
		Severity:    ,
		Status:      "Open",
		CreatedAt:   now.UnixMilli(),
		UpdatedAt:   now.UnixMilli(),
	}

	incident := adapter.transformCase(theHiveCase)

	assert.Equal(t, "Security Incident", incident.Title)
	assert.Equal(t, "Detailed description", incident.Description)
	assert.Equal(t, "HIGH", incident.Severity)
	assert.Equal(t, "Open", incident.Status)
	assert.Equal(t, "case_", incident.ExternalID)
	assert.Equal(t, "THEHIVE", incident.Source)
	assert.NotZero(t, incident.ID)
}

func TestMockIncidents(t testing.T) {
	adapter := NewTheHiveAdapter(config.ExternalService{})

	mockInc := adapter.mockIncidents()

	assert.Equal(t, , len(mockInc))

	for _, inc := range mockInc {
		assert.NotZero(t, inc.ID)
		assert.NotEmpty(t, inc.Title)
		assert.NotEmpty(t, inc.Description)
		assert.NotEmpty(t, inc.Severity)
		assert.Equal(t, "THEHIVE", inc.Source)
	}
}

func TestFetchRecentIncidentsFiltersClosedCases(t testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r http.Request) {
		response := TheHiveResponse{
			Success: true,
			Data: []TheHiveCase{
				{
					ID:        "open_case",
					Title:     "Open Case",
					Severity:  ,
					Status:    "Open",
					CreatedAt: time.Now().Unix()  ,
				},
				{
					ID:        "closed_case",
					Title:     "Closed Case",
					Severity:  ,
					Status:    "Closed",
					CreatedAt: time.Now().Unix()  ,
				},
				{
					ID:        "in_progress_case",
					Title:     "In Progress Case",
					Severity:  ,
					Status:    "In Progress",
					CreatedAt: time.Now().Unix()  ,
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	cfg := config.ExternalService{
		Enabled: true,
		URL:     server.URL,
		APIKey:  "test-key",
	}

	adapter := NewTheHiveAdapter(cfg)

	incidents, err := adapter.FetchRecentIncidents()

	assert.NoError(t, err)
	assert.Equal(t, , len(incidents))
	assert.Equal(t, "Open Case", incidents[].Title)
	assert.Equal(t, "In Progress Case", incidents[].Title)
}

func TestAPIAuthenticationHeader(t testing.T) {
	headerCaptured := false
	correctAuth := false

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "" {
			headerCaptured = true
			correctAuth = auth == "Bearer test-secret-key"
		}

		response := TheHiveResponse{
			Success: true,
			Data:    []TheHiveCase{},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	cfg := config.ExternalService{
		Enabled: true,
		URL:     server.URL,
		APIKey:  "test-secret-key",
	}

	adapter := NewTheHiveAdapter(cfg)

	incidents, err := adapter.FetchRecentIncidents()

	assert.NoError(t, err)
	assert.True(t, headerCaptured, "Authorization header should be sent")
	assert.True(t, correctAuth, "Authorization header should contain correct API key")
	require.NotNil(t, incidents)
}
