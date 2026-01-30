 Sync Engine Documentation

 Overview

The Sync Engine is a production-grade background worker that continuously synchronizes incidents from external sources (TheHive, OpenCTI, OpenRMF) into OpenRisk risk records. It implements enterprise-level reliability patterns including exponential backoff retry logic, structured JSON logging, and metrics collection.

 Architecture

 Components

. SyncEngine - Main orchestration component
   - Manages background sync loop with configurable intervals
   - Implements graceful shutdown with context cancellation
   - Tracks synchronization metrics and health status
   - Coordinates incident processing pipeline

. SyncMetrics - Observability metrics
   - Total sync attempts, successful/failed counts
   - Incident creation/update counts
   - Last sync time and error tracking
   - Thread-safe operations with RWMutex

. Adapters - External integration providers
   - TheHive (incident integration)  Implemented
   - OpenCTI (threat intelligence)  Planned
   - OpenRMF (compliance controls)  Planned

 Features

 . Exponential Backoff Retry Logic

When API calls fail, the engine automatically retries with exponential backoff:

- Retry count: Configurable (default:  retries)
- Initial backoff:  second
- Backoff progression: s → s → s → s (capped at maxBackoff)
- Use case: Handles transient network failures, API rate limits, temporary service unavailability


Attempt  (immediate): Fails
  Wait s...
Attempt : Fails
  Wait s...
Attempt : Fails
  Wait s...
Attempt : Success 


 . Structured JSON Logging

All log output is JSON-formatted for easy parsing by log aggregation systems:

json
{
  "timestamp": "--T::+:",
  "level": "INFO",
  "component": "sync_engine",
  "message": "Incident sync cycle completed",
  "duration_ms": ,
  "incidents_total": ,
  "processed": 
}


Log levels:
- INFO - Normal operational events (sync start/complete, startup/shutdown)
- WARN - Retry attempts after failures
- ERROR - Final failure after all retries exhausted
- DEBUG - Detailed incident processing logs

 . Metrics Collection

Real-time metrics tracking for observability:

go
type SyncMetrics struct {
    TotalSyncs       int         // Total sync attempts
    SuccessfulSyncs  int         // Successful cycles
    FailedSyncs      int         // Failed after all retries
    IncidentsCreated int         // New risks created
    IncidentsUpdated int         // Existing risks updated
    LastSyncTime     time.Time     // Last successful sync
    LastError        string        // Most recent error message
    LastErrorTime    time.Time     // When last error occurred
}


Access metrics via:
go
metrics := engine.GetMetrics()
fmt.Printf("Success rate: %.f%%\n", 
    float(metrics.SuccessfulSyncs) / float(metrics.TotalSyncs)  )


 . TheHive Adapter Integration

 Real API Calls

The adapter connects to TheHive using the official REST API:


GET /api/case?limit=&sort=-createdAt
Authorization: Bearer {API_KEY}


 Features

-  Real HTTP API calls with authentication
-  Case status filtering (excludes Closed/Resolved)
-  Severity mapping (- → LOW/MEDIUM/HIGH/CRITICAL)
-  Pagination support (limit=)
-  Error handling with fallback to mock data
-  Network timeout handling (s HTTP timeout)
-  Connection pooling and keep-alive

 Configuration

go
cfg := config.ExternalService{
    Enabled: true,
    URL:     "http://thehive:",
    APIKey:  "your-api-key-here",
}
adapter := NewTheHiveAdapter(cfg)


 Incident Transformation

Raw TheHive case:
json
{
  "id": "case_",
  "title": "Ransomware Detected",
  "description": "Encrypted files on HR Server",
  "severity": ,
  "status": "Open"
}


Transformed to Risk:
json
{
  "title": "[INCIDENT] Ransomware Detected",
  "description": "Auto-created from incident case_\n\nEncrypted files on HR Server",
  "impact": ,
  "probability": ,
  "source": "THEHIVE",
  "external_id": "case_",
  "tags": ["INCIDENT", "AUTOMATED", "HIGH"]
}


 . Graceful Lifecycle Management

 Starting the Engine

go
ctx := context.Background()
engine := NewSyncEngine(adapter)
engine.Start(ctx)
// Sync loop runs in background goroutine


 Configuration

go
engine := &SyncEngine{
    IncidentProvider: adapter,
    maxRetries:       ,         // Max retry attempts
    initialBackoff:     time.Second,  // s-s exponential
    maxBackoff:         time.Second,
    syncInterval:       time.Minute,  // Run every minute
}


 Graceful Shutdown

go
// Cancel context to stop sync loop
cancel()
// OR manually stop
engine.Stop()

// Wait for goroutine to finish (before server shutdown)
<-engine.doneCh


 Startup Behavior

. Engine immediately runs first sync cycle (cold start)
. Then waits for ticker interval before next cycle
. Logs startup event with configured interval
. Returns control to caller (runs in background)

 Incident Processing Pipeline



 Sync Cycle Starts   

           

 FetchRecentIncidents (with retry logic)

           

 Process Each        
 Incident            

           
        
             
    HIGH/  LOW/
   CRITICAL MEDIUM
             
      Skip (no-op)
          
 Transform to Risk
          
    
       

CreateRiskIfNot   (idempotency via ExternalID)
Exists           

       

Update Metrics   

       

Log Completion   



 Error Handling

 Transient Errors (Network/Timeout)

Automatically retried with exponential backoff:
- Connection timeouts
- Service temporarily unavailable
- Rate limit responses ()

 Permanent Errors

Logged but don't block subsequent syncs:
- Database write failures
- Invalid incident data
- Misconfigured API credentials

 Graceful Degradation

If TheHive API unavailable:
. Initial sync attempt fails
. Retries with backoff
. Falls back to mock data (for development)
. Continues with next scheduled sync
. No service disruption

 Testing

 Unit Tests ( tests, % coverage)

Sync Engine Tests ( tests)
-  Initialization with correct defaults
-  Metrics tracking across sync cycles
-  Exponential backoff retry behavior ( attempts)
-  Failure handling after all retries exhausted
-  Low severity incident skipping
-  High severity incident processing
-  Graceful start/stop lifecycle
-  Structured JSON logging output
-  Severity mapping logic
-  Thread-safe concurrent metrics access
-  Context cancellation (graceful shutdown)

TheHive Adapter Tests ( tests)
-  Adapter initialization with config
-  Disabled adapter returns empty list
-  Mock data fallback when no API configured
-  Real API calls with authentication
-  HTTP error handling (, , etc.)
-  Network timeout handling
-  Severity mapping (- → domain values)
-  Complete case transformation
-  Mock data structure verification
-  Closed case filtering
-  Authorization header verification

Run tests:
bash
go test -v ./internal/workers/... ./internal/adapters/thehive/...


 Integration Tests

Full end-to-end tests with real PostgreSQL database are in:
- backend/internal/handlers/risk_handler_integration_test.go

These test the complete pipeline from incident creation to database persistence.

 Configuration

 Environment Variables

bash
 TheHive Integration
THEHIVE_ENABLED=true
THEHIVE_URL=http://thehive:
THEHIVE_API_KEY=your-secret-api-key

 OpenCTI Integration (upcoming)
OPENCTI_ENABLED=false
OPENCTI_URL=http://opencti:
OPENCTI_API_KEY=

 OpenRMF Integration (upcoming)
OPENRMF_ENABLED=false
OPENRMF_URL=http://openrmf:
OPENRMF_API_KEY=


 Runtime Configuration

go
cfg := &config.Config{
    TheHive: config.ExternalService{
        Enabled: os.Getenv("THEHIVE_ENABLED") == "true",
        URL:     os.Getenv("THEHIVE_URL"),
        APIKey:  os.Getenv("THEHIVE_API_KEY"),
    },
}

engine := NewSyncEngine(
    thehive.NewTheHiveAdapter(cfg.TheHive),
)


 Monitoring & Observability

 Health Checks

go
metrics := engine.GetMetrics()

if metrics.FailedSyncs >  {
    // Alert - high failure rate
}

if time.Since(metrics.LastSyncTime) > time.Minute {
    // Alert - sync appears stuck
}


 Metrics Exposure

Currently output to stdout as JSON. Future integrations:
- Prometheus /metrics endpoint
- ELK stack integration for log aggregation
- Grafana dashboards
- Alert Manager integration

 Key Metrics for Monitoring

. Sync Success Rate = SuccessfulSyncs / TotalSyncs
. Incident Throughput = IncidentsCreated + IncidentsUpdated per hour
. Error Rate = FailedSyncs / TotalSyncs
. Sync Latency = Duration of syncIncidents()
. Age of Last Sync = Now() - LastSyncTime

 Future Enhancements

 Planned Features

. Metrics Export
   - Prometheus metrics endpoint
   - StatsD integration
   - CloudWatch metrics

. Advanced Retry Logic
   - Circuit breaker pattern
   - Jitter to prevent thundering herd
   - Dead letter queue for failed incidents

. Connectors
   - OpenCTI (threat intelligence)
   - OpenRMF (compliance)
   - Splunk (SIEM)
   - Elastic (ELK)
   - Cortex (automated response)

. Performance
   - Parallel incident processing
   - Batch database operations
   - Incremental sync (since last successful sync)
   - Change data capture (CDC)

. Reliability
   - Idempotency guarantees
   - At-least-once delivery semantics
   - Distributed tracing (OpenTelemetry)
   - Semantic versioning for APIs

 Troubleshooting

 Issue: Sync not running

Check: Is context cancelled?
go
select {
case <-ctx.Done():
    // Engine stopped
}


Check: Is engine started?
go
metrics := engine.GetMetrics()
if metrics.TotalSyncs ==  {
    // Not running
}


 Issue: High failure rate

Check: API credentials

THEHIVE_API_KEY=your-actual-key


Check: Network connectivity
bash
curl -H "Authorization: Bearer key" http://thehive:/api/case


Check: Logs for error details
json
{"level":"ERROR","message":"API returned status "}


 Issue: Database write failures

Check: Risk schema matches
sql
SELECT COUNT() FROM risks WHERE source = 'THEHIVE';


Check: Foreign key constraints
sql
SELECT  FROM public.risks WHERE source = 'THEHIVE';


 Performance Characteristics

- Memory: ~-MB per engine instance
- CPU: <% idle, -% during sync
- Sync Duration: ms-s depending on incident count
- Throughput: - incidents per minute

 Security Considerations

. API Key Storage
   - Never log API keys
   - Use environment variables or secrets manager
   - Rotate keys regularly

. Network Security
   - Use HTTPS in production
   - Certificate validation enabled by default
   - s HTTP timeout prevents hanging connections

. Database Security
   - Prepared statements prevent SQL injection
   - Row-level security via user context
   - Audit logging of risk changes

. Input Validation
   - Case title/description sanitized
   - External IDs validated as UUIDs
   - Severity enum validated

 References

- TheHive API: https://docs.thehive-project.org/
- OpenCTI API: https://github.com/OpenCTI-Platform/opencti
- OpenRMF API: https://github.com/Defense-Counterintelligence-and-Security-Agency/openrmf-api
- Go Context: https://golang.org/pkg/context/
- Go Ticker: https://golang.org/pkg/time/Ticker
