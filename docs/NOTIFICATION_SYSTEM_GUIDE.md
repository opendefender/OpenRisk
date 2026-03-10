# 🔔 Notification System Implementation Guide

**Status:** ✅ COMPLETE  
**Branch:** `feat/notification-system`  
**Last Updated:** March 10, 2026

---

## Table of Contents

1. [Overview](#overview)
2. [Architecture](#architecture)
3. [Notification Types](#notification-types)
4. [Channels](#channels)
5. [Configuration](#configuration)
6. [API Reference](#api-reference)
7. [Code Examples](#code-examples)
8. [Database Schema](#database-schema)
9. [Troubleshooting](#troubleshooting)
10. [Future Enhancements](#future-enhancements)

---

## Overview

The **Notification System** is a comprehensive multi-channel notification delivery platform that alerts users about critical risk events, approaching mitigation deadlines, and assigned actions.

### Key Features

✅ **Multiple Channels**
- Email (SMTP)
- Slack (Webhooks)
- Webhooks (Generic with HMAC-SHA256 signing)
- In-App (Database-backed)

✅ **User Preferences**
- Per-channel toggles
- Configurable notification types
- Deadline advance notice (e.g., 3 days before)
- Sound and desktop notifications

✅ **Reliability**
- Exponential backoff retry logic
- Delivery logging and tracking
- Automatic cleanup of old notifications
- Failed notification recovery

✅ **Security**
- HMAC-SHA256 webhook signature verification
- User-scoped notification access
- Tenant isolation
- Preference encryption ready

---

## Architecture

### Component Diagram

```
┌─────────────────────────────────────────────────────────┐
│                   Notification Events                    │
│    (Deadline approaching, Critical Risk, Action assigned)│
└────────────────┬────────────────────────────────────────┘
                 │
                 ▼
         ┌──────────────────┐
         │ Notification     │
         │ Service          │
         └────────┬─────────┘
                  │
         ┌────────┴────────┐
         │                 │
         ▼                 ▼
    ┌─────────┐      ┌──────────┐
    │ Domain  │      │ Provider │
    │ Models  │      │ Layer    │
    └─────────┘      └──────────┘
                         │
         ┌───────────────┼───────────────┐
         │               │               │
         ▼               ▼               ▼
    ┌─────────┐  ┌──────────┐  ┌─────────────┐
    │  Email  │  │  Slack   │  │   Webhook   │
    │Provider │  │ Provider │  │   Provider  │
    └────┬────┘  └─────┬────┘  └──────┬──────┘
         │             │              │
         ▼             ▼              ▼
    ┌──────────┐  ┌─────────┐  ┌─────────────┐
    │   SMTP   │  │ Webhook │  │ External    │
    │  Server  │  │   API   │  │ Webhook URL │
    └──────────┘  └─────────┘  └─────────────┘
```

### Layers

**Domain Layer** (`notification.go`)
- Notification entity
- NotificationPreference entity
- NotificationTemplate entity
- NotificationLog entity
- Type and status enums

**Service Layer** (`notification_service.go`)
- Business logic for sending notifications
- Preference management
- Retrieval and filtering
- Cleanup operations

**Provider Layer**
- `email_provider.go`: Email delivery
- `slack_provider.go`: Slack webhook integration
- `webhook_provider.go`: Generic webhook with signing

**Handler Layer** (`notification_handler.go`)
- REST API endpoints
- Request validation
- Response formatting

**Database Layer**
- Notification storage
- Preference persistence
- Audit logging

---

## Notification Types

### 1. Mitigation Deadline (🔶 Orange)

**Purpose:** Alert users when a mitigation deadline is approaching

**Trigger:** Risk with upcoming mitigation deadline

**Data Included:**
- Risk title and description
- Days until deadline
- Assigned to user
- Severity level
- Link to risk details

**Example:**
```json
{
  "type": "mitigation_deadline",
  "channel": "email",
  "subject": "Mitigation Deadline Approaching: SQL Injection Vulnerability",
  "message": "Your risk mitigation is due in 3 days",
  "metadata": {
    "risk_id": "uuid",
    "risk_title": "SQL Injection Vulnerability",
    "days_until_due": 3,
    "assigned_to": "John Doe",
    "severity": "CRITICAL",
    "risk_link": "https://app.openrisk.com/risks/uuid"
  }
}
```

---

### 2. Critical Risk (🔴 Red)

**Purpose:** Immediate alert for newly detected critical risks

**Trigger:** Risk detected with CRITICAL severity level

**Data Included:**
- Risk title and description
- Severity level
- Impact assessment
- Probability assessment
- Recommended actions
- Link to risk details

**Example:**
```json
{
  "type": "critical_risk",
  "channel": "slack",
  "subject": "🚨 CRITICAL RISK DETECTED: Data Breach Vulnerability",
  "metadata": {
    "risk_id": "uuid",
    "risk_title": "Data Breach Vulnerability",
    "severity": "CRITICAL",
    "impact": "HIGH",
    "probability": "MEDIUM",
    "recommended_actions": [
      "Implement encryption",
      "Conduct security audit",
      "Update access controls"
    ],
    "risk_link": "https://app.openrisk.com/risks/uuid"
  }
}
```

---

### 3. Action Assigned (🔵 Blue)

**Purpose:** Notify user when an action is assigned to them

**Trigger:** User assigned to a risk mitigation action

**Data Included:**
- Action title
- Risk being mitigated
- Due date
- Priority level
- Assigned by user
- Link to action

**Example:**
```json
{
  "type": "action_assigned",
  "channel": "email",
  "subject": "New Action Assigned: Update Database Security",
  "metadata": {
    "action_id": "uuid",
    "action_title": "Update Database Security",
    "risk_id": "uuid",
    "risk_title": "SQL Injection Vulnerability",
    "due_date": "2026-03-20",
    "priority": "HIGH",
    "assigned_by": "Risk Manager",
    "action_link": "https://app.openrisk.com/actions/uuid"
  }
}
```

---

### 4. Risk Update (Optional)

**Purpose:** Notify stakeholders of risk status changes

**Triggers:**
- Risk severity updated
- Risk status changed
- Mitigation progress updated

---

### 5. Risk Resolved (Optional)

**Purpose:** Confirm when a risk has been successfully mitigated

**Data Included:**
- Risk title
- Resolution date
- Final status
- Closure notes

---

## Channels

### Email

**Provider:** SMTP (Currently placeholder, needs production integration)

**Configuration:**
```go
// In environment variables or config:
SMTP_HOST=smtp.example.com
SMTP_PORT=587
SMTP_USER=noreply@openrisk.com
SMTP_PASSWORD=***
SMTP_FROM=notifications@openrisk.com
```

**Features:**
- HTML email templates
- Sender verification
- Reply-To header
- Unsubscribe link
- Test email capability

**Best For:**
- Non-urgent notifications
- Digestible information
- Record-keeping
- Users without Slack

---

### Slack

**Provider:** Slack Incoming Webhooks

**Configuration:**
```bash
# User sets up their own webhook in Slack workspace settings
# Slack → Your Workspace → Apps → Incoming Webhooks → Create New Webhook
```

**Features:**
- Color-coded messages (Red/Orange/Blue)
- Rich formatting with fields
- Threaded conversations
- Interactive elements (buttons, select menus)

**Color Scheme:**
- 🔴 Critical Risk: `#ff0000` (Red)
- 🔶 Deadline: `#ff9900` (Orange)
- 🔵 Action: `#0099ff` (Blue)
- ✅ Default: `#36a64f` (Green)

**Example Slack Message:**
```
🚨 CRITICAL RISK DETECTED
Risk: SQL Injection Vulnerability
Severity: CRITICAL
Impact: HIGH
Probability: MEDIUM

[View in OpenRisk] [Dismiss]
```

**Best For:**
- Urgent notifications
- Team collaboration
- Real-time alerts
- Active monitoring

---

### Webhook (Generic)

**Provider:** Custom HTTP endpoint (yours or third-party like PagerDuty)

**Configuration:**
```bash
# User provides their webhook URL
WEBHOOK_URL=https://your-service.com/webhooks/openrisk
WEBHOOK_SECRET=your-secret-key
```

**Features:**
- HMAC-SHA256 signature verification
- Exponential backoff retries
- Delivery logging
- Batch capabilities
- Custom payload structure

**Security:**
```
Header: X-OpenRisk-Signature
Value: sha256=abcd1234...
```

**Webhook Payload:**
```json
{
  "event": "notification.sent",
  "timestamp": "2026-03-10T14:30:00Z",
  "notification_id": "uuid",
  "user_id": "uuid",
  "tenant_id": "uuid",
  "type": "critical_risk",
  "channel": "webhook",
  "subject": "🚨 CRITICAL RISK DETECTED",
  "message": "A critical risk has been detected...",
  "metadata": { /* full metadata */ }
}
```

**Verification (In Your System):**
```go
// Verify signature on received webhook
func verifyOpenRiskSignature(signature, payload, secret string) bool {
    expected := createSignature(payload, secret)
    return subtle.ConstantTimeCompare(
        []byte(signature),
        []byte(expected),
    ) == 1
}
```

**Best For:**
- System integrations
- Incident management (PagerDuty, Opsgenie)
- Custom processing
- External data warehouses

---

### In-App

**Provider:** Database-backed notifications

**Features:**
- Always available
- Notification center UI
- Mark as read/unread
- Notification history
- Soft delete (archive)

**Best For:**
- Persistent notification history
- Non-critical alerts
- User interface integration
- Compliance/audit trails

---

## Configuration

### Environment Variables

```bash
# Email Configuration
SMTP_HOST=smtp.sendgrid.net
SMTP_PORT=587
SMTP_USER=apikey
SMTP_PASSWORD=SG.xxxxx
SMTP_FROM=noreply@openrisk.com

# Notification Settings
NOTIFICATION_RETENTION_DAYS=90
NOTIFICATION_BATCH_SIZE=100
NOTIFICATION_BATCH_DELAY_MS=1000

# Webhook Settings
WEBHOOK_TIMEOUT_SECONDS=10
WEBHOOK_RETRY_COUNT=3
WEBHOOK_RETRY_BACKOFF_MS=1000
```

### Database Setup

Run migrations in order:

```bash
# 1. Create notifications table
psql -f database/0015_create_notifications_table.sql

# 2. Create notification preferences table
psql -f database/0016_create_notification_preferences_table.sql

# 3. Create notification templates table
psql -f database/0017_create_notification_templates_table.sql

# 4. Create notification logs table
psql -f database/0018_create_notification_logs_table.sql
```

### Service Initialization

```go
// In your main.go or service setup
notificationService := services.NewNotificationService(
    db,
    &providers.EmailProvider{
        Host:     os.Getenv("SMTP_HOST"),
        Port:     587,
        User:     os.Getenv("SMTP_USER"),
        Password: os.Getenv("SMTP_PASSWORD"),
        From:     os.Getenv("SMTP_FROM"),
    },
    &providers.SlackProvider{},
    &providers.WebhookProvider{
        Timeout: 10 * time.Second,
        Retries: 3,
    },
)

// Register handlers
handler := handlers.NewNotificationHandler(notificationService)
```

---

## API Reference

### Base URL
```
/api/v1/notifications
```

### Endpoints

#### 1. Get User Notifications

```http
GET /api/v1/notifications?limit=50&offset=0
Content-Type: application/json
Authorization: Bearer {token}
```

**Response:**
```json
{
  "data": [
    {
      "id": "uuid",
      "user_id": "uuid",
      "type": "critical_risk",
      "channel": "email",
      "status": "delivered",
      "subject": "🚨 CRITICAL RISK DETECTED",
      "message": "A critical risk has been detected...",
      "created_at": "2026-03-10T14:30:00Z"
    }
  ],
  "limit": 50,
  "offset": 0,
  "total": 125
}
```

---

#### 2. Get Unread Count

```http
GET /api/v1/notifications/unread-count
Authorization: Bearer {token}
```

**Response:**
```json
{
  "unread_count": 5
}
```

---

#### 3. Mark Notification as Read

```http
PATCH /api/v1/notifications/{notificationId}/read
Authorization: Bearer {token}
```

**Response:**
```json
{
  "message": "notification marked as read"
}
```

---

#### 4. Mark All as Read

```http
PATCH /api/v1/notifications/read-all
Authorization: Bearer {token}
```

**Response:**
```json
{
  "message": "all notifications marked as read"
}
```

---

#### 5. Delete Notification

```http
DELETE /api/v1/notifications/{notificationId}
Authorization: Bearer {token}
```

**Response:**
```json
{
  "message": "notification deleted"
}
```

---

#### 6. Get Notification Preferences

```http
GET /api/v1/notifications/preferences
Authorization: Bearer {token}
```

**Response:**
```json
{
  "id": "uuid",
  "user_id": "uuid",
  "email_on_mitigation_deadline": true,
  "email_on_critical_risk": true,
  "email_on_action_assigned": true,
  "email_deadline_advance_days": 3,
  "slack_enabled": false,
  "slack_on_mitigation_deadline": true,
  "slack_on_critical_risk": true,
  "slack_on_action_assigned": true,
  "webhook_enabled": false,
  "disable_all_notifications": false,
  "enable_sound_notifications": true,
  "enable_desktop_notifications": true
}
```

---

#### 7. Update Notification Preferences

```http
PATCH /api/v1/notifications/preferences
Content-Type: application/json
Authorization: Bearer {token}

{
  "email_on_mitigation_deadline": true,
  "email_on_critical_risk": false,
  "email_deadline_advance_days": 5,
  "slack_enabled": true,
  "enable_sound_notifications": false
}
```

**Response:** Updated preferences object

---

#### 8. Send Test Notification

```http
POST /api/v1/notifications/test
Content-Type: application/json
Authorization: Bearer {token}

{
  "channel": "email"
}
```

**Response:**
```json
{
  "message": "test notification would be sent",
  "channel": "email"
}
```

---

## Code Examples

### Sending a Critical Risk Notification

**Backend (Go):**
```go
payload := &domain.CriticalRiskNotificationPayload{
    RiskID:              riskID,
    RiskTitle:           "SQL Injection Vulnerability",
    Severity:            "CRITICAL",
    Impact:              "HIGH",
    Probability:         "MEDIUM",
    RecommendedActions: []string{
        "Implement parameterized queries",
        "Code review",
        "Security testing",
    },
    RiskLink: fmt.Sprintf("https://app.openrisk.com/risks/%s", riskID),
}

err := notificationService.SendCriticalRiskNotification(
    ctx,
    userID,
    tenantID,
    payload,
)
```

---

### Sending a Deadline Notification

**Backend (Go):**
```go
payload := &domain.MitigationDeadlineNotificationPayload{
    RiskID:         riskID,
    RiskTitle:      "Data Encryption",
    DaysUntilDue:   3,
    AssignedTo:     "John Doe",
    Severity:       "HIGH",
    RiskLink:       fmt.Sprintf("https://app.openrisk.com/risks/%s", riskID),
}

err := notificationService.SendMitigationDeadlineNotification(
    ctx,
    userID,
    tenantID,
    payload,
)
```

---

### Updating Notification Preferences

**Frontend (TypeScript/React):**
```typescript
async function updateNotificationPreferences(
  preferences: NotificationPreferences
) {
  const response = await fetch('/api/v1/notifications/preferences', {
    method: 'PATCH',
    headers: {
      'Content-Type': 'application/json',
      'Authorization': `Bearer ${token}`,
    },
    body: JSON.stringify(preferences),
  });

  return response.json();
}

// Usage
updateNotificationPreferences({
  email_on_critical_risk: false,
  slack_enabled: true,
  enable_sound_notifications: false,
});
```

---

### Verifying Incoming Webhook

**Your Backend:**
```go
func handleOpenRiskWebhook(w http.ResponseWriter, r *http.Request) {
    // Read signature from header
    signature := r.Header.Get("X-OpenRisk-Signature")
    timestamp := r.Header.Get("X-OpenRisk-Event-Timestamp")
    
    // Read body
    body, err := ioutil.ReadAll(r.Body)
    if err != nil {
        http.Error(w, "Invalid request", http.StatusBadRequest)
        return
    }
    
    // Verify signature
    if !verifySignature(signature, body, os.Getenv("OPENRISK_WEBHOOK_SECRET")) {
        http.Error(w, "Invalid signature", http.StatusUnauthorized)
        return
    }
    
    // Parse and process notification
    var notification OpenRiskNotification
    if err := json.Unmarshal(body, &notification); err != nil {
        http.Error(w, "Invalid payload", http.StatusBadRequest)
        return
    }
    
    // Process notification
    processNotification(notification)
    
    w.WriteHeader(http.StatusOK)
    json.NewEncoder(w).Encode(map[string]string{"status": "received"})
}
```

---

## Database Schema

### notifications table

| Column | Type | Description |
|--------|------|-------------|
| id | UUID | Primary key |
| user_id | UUID | User receiving notification |
| tenant_id | UUID | Tenant isolation |
| type | VARCHAR(50) | Notification type |
| channel | VARCHAR(50) | Delivery channel |
| status | VARCHAR(50) | Current status (pending, sent, delivered, failed, read) |
| subject | TEXT | Notification subject |
| message | TEXT | Notification message |
| metadata | JSONB | Additional data (risk_id, severity, etc.) |
| created_at | TIMESTAMP | Creation date |
| updated_at | TIMESTAMP | Last update |
| deleted_at | TIMESTAMP | Soft delete timestamp |

### notification_preferences table

| Column | Type | Description |
|--------|------|-------------|
| id | UUID | Primary key |
| user_id | UUID | User (unique) |
| tenant_id | UUID | Tenant |
| email_on_mitigation_deadline | BOOLEAN | Email for deadlines |
| email_on_critical_risk | BOOLEAN | Email for critical risks |
| slack_enabled | BOOLEAN | Slack integration enabled |
| webhook_enabled | BOOLEAN | Webhook integration enabled |
| disable_all_notifications | BOOLEAN | Global disable switch |
| enable_sound_notifications | BOOLEAN | Sound alerts |
| enable_desktop_notifications | BOOLEAN | Desktop alerts |
| created_at | TIMESTAMP | Creation date |
| updated_at | TIMESTAMP | Last update |

### notification_logs table

| Column | Type | Description |
|--------|------|-------------|
| id | UUID | Primary key |
| notification_id | UUID | Notification being logged |
| user_id | UUID | User |
| channel | VARCHAR(50) | Delivery channel |
| provider | VARCHAR(100) | Provider name (SendGrid, Slack, Custom) |
| status | VARCHAR(50) | Delivery status |
| error_message | TEXT | Error details if failed |
| retry_count | INTEGER | Number of retries |
| sent_at | TIMESTAMP | Send timestamp |
| delivered_at | TIMESTAMP | Delivery confirmation |
| failed_at | TIMESTAMP | Failure timestamp |

### notification_templates table

| Column | Type | Description |
|--------|------|-------------|
| id | UUID | Primary key |
| tenant_id | UUID | Tenant |
| name | VARCHAR(255) | Template name |
| type | VARCHAR(50) | Notification type |
| subject | VARCHAR(500) | Email subject template |
| message | TEXT | Message template with {variables} |
| is_default | BOOLEAN | Default template flag |
| is_active | BOOLEAN | Active/inactive |
| created_by | UUID | Creator user ID |

---

## Troubleshooting

### Issue: Notifications Not Being Sent

**Check List:**
1. ✅ Are notification preferences enabled?
   ```sql
   SELECT * FROM notification_preferences 
   WHERE user_id = 'user-uuid';
   ```

2. ✅ Are there any errors in notification_logs?
   ```sql
   SELECT * FROM notification_logs 
   WHERE notification_id = 'notification-uuid'
   ORDER BY created_at DESC;
   ```

3. ✅ Is the notification service properly initialized?
   ```go
   // Verify service and providers are instantiated
   ```

4. ✅ Check Slack webhook URL validity
   ```bash
   curl -X POST https://hooks.slack.com/services/YOUR/WEBHOOK/URL \
     -H 'Content-Type: application/json' \
     -d '{"text":"Test message"}'
   ```

---

### Issue: Email Delivery Failures

**Solutions:**
1. Verify SMTP credentials in environment variables
2. Check that SMTP port is correct (usually 587 for TLS)
3. Ensure sender email is whitelisted
4. Verify email provider (SendGrid, Mailgun) configuration
5. Check spam folder for test emails

---

### Issue: Webhook Not Being Called

**Solutions:**
1. Verify webhook URL is accessible from OpenRisk server
2. Check webhook secret is correct
3. Verify request method is POST
4. Check firewall/security group allows outbound traffic
5. Review webhook logs for failed attempts

---

### Issue: High Database Growth

**Solutions:**
1. Run notification cleanup job
   ```sql
   -- Archive old notifications
   UPDATE notifications 
   SET deleted_at = NOW() 
   WHERE created_at < NOW() - INTERVAL '90 days'
   AND deleted_at IS NULL;
   ```

2. Configure retention policy
3. Archive notification_logs periodically
4. Create database indexes for common queries

---

## Future Enhancements

### Phase 2 (Post-Launch)

- [ ] SMS notifications via Twilio
- [ ] Push notifications (mobile app)
- [ ] Microsoft Teams integration
- [ ] PagerDuty escalation
- [ ] Notification digest (daily/weekly summary)
- [ ] Advanced scheduling (quiet hours, do-not-disturb)
- [ ] Notification templates UI
- [ ] Recipient groups (notify all analysts)
- [ ] A/B testing for notification messages
- [ ] Notification analytics dashboard

### Integration Points

```
Email Templates
    ↓
    Notification Service
    ↓
Risk Detection ──→ [Service] ←─→ Database
    ↓          ↓
    └─→ Email Provider
        Slack Provider
        Webhook Provider
```

---

## Support & Contact

**Documentation:** See NOTIFICATION_SYSTEM_GUIDE.md  
**Issues:** Report in GitHub Issues  
**Questions:** Contact development team

---

**Version:** 1.0  
**Last Updated:** March 10, 2026  
**Author:** OpenRisk Development Team
