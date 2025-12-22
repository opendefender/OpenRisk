# OpenRisk - Quick Start Guide

Welcome! This guide gets you up and running in **5 minutes** with realistic sample data.

---

## ⚡ Step 1: Start the System (2 min)

### Prerequisites
- Docker & Docker Compose installed
- Git
- A terminal (Bash, Zsh, PowerShell, etc.)

### Launch OpenRisk

```bash
# 1. Clone the repository
git clone https://github.com/alex-dembele/OpenRisk.git
cd OpenRisk

# 2. Start all services
docker compose up -d

# 3. Verify everything is running
docker compose ps
# Should show: db, redis, backend, frontend (all UP)

# 4. Access the interface
# → Frontend: http://localhost:5173
# → API Backend: http://localhost:8080
```

### ✅ Health Check

```bash
# Verify services are responding
curl http://localhost:8080/health
# Expected result: {"status":"healthy"}
```

---

## 🔐 Step 2: Login (1 min)

### Default Credentials
```
📧 Email: admin@openrisk.local
🔑 Password: admin123
```

### First Login

1. Open http://localhost:5173 in your browser
2. Enter the credentials above
3. Click "Login"

**You're now on the Dashboard!**

---

## 📊 Step 3: Explore the Dashboard (30 sec)

You'll see 4 sections:

### 📈 Top Left: Overview
```
8 High Risks
12 Medium Risks
5 Low Risks
```

### 📉 Top Right: Trend Chart
```
Shows risk evolution over the last 30 days
(Currently empty, we'll add data next)
```

### 🗺️ Bottom Left: Heatmap
```
Probability vs Impact matrix
Visualize risks visually
```

### 📋 Bottom Right: Recent Risks
```
List of recently created risks
(Currently empty)
```

---

## 📥 Step 4: Import Test Data (2 min)

### Option A: Import via API (Recommended)

**Download test file:**

```bash
# File is included in the repo
cat dev/fixtures/risks.json
```

**Import the data:**

```bash
# Option 1: Via cURL (command line)
curl -X POST http://localhost:8080/api/risks/bulk-import \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d @dev/fixtures/risks.json

# Option 2: Via interface (easier)
# 1. Go to Settings → Data Management
# 2. Click "Import Data"
# 3. Upload dev/fixtures/risks.json
# 4. Click "Import"
```

### Option B: Create a Risk Manually

1. Click "Risks" in the menu
2. Click "Create Risk"
3. Fill the form:

```
Title: SQL Injection vulnerability in login form
Description: User input is not escaped
Framework: OWASP Top 10 - A03:2021 Injection
Criticality: High
Probability: Medium
Status: Identified

Auto-calculated Score: 7.5/10 ✅
```

4. Click "Save"

---

## 🛡️ Step 5: Create a Mitigation (2 min)

### From an Existing Risk

1. Click a risk (e.g., "SQL Injection")
2. Go to "Mitigations" tab
3. Click "Add Mitigation"
4. Fill:

```
Title: Use Prepared Statements
Description: Refactor database layer
Status: In Progress
Owner: Backend Team Lead
Deadline: January 15, 2026
```

### Add Sub-Actions (Checklist)

```
Sub-actions:
☐ Validate with security team
☐ Write unit tests
☐ Deploy to staging
☐ Test for 24 hours in production
☐ Monitor logs
```

**Check as you go:**
```bash
# When action is done, click ☐ → ☑️
# System auto-tracks progress
```

---

## 📊 Step 6: Generate a Report (1 min)

### Create a Simple Report

1. Click "Reports" in menu
2. Click "Create Report"
3. Select:
   - **Type**: Risk Summary
   - **Period**: This Month
   - **Format**: PDF
4. Click "Generate"

**Report generated in 10 seconds!**

### What's in the Report

```
📊 RISK MANAGEMENT REPORT
Generated: December 22, 2025

Summary:
- Total risks: 3
- Critical: 1
- High: 1
- Medium: 1

Details:
1. SQL Injection (Score: 7.5) → Mitigation in progress
2. ...

Recommended Actions:
- Accelerate Critical mitigation
- ...
```

---

## 🔌 Step 7: Connect Your Tools (Optional)

### Splunk Integration

If you use Splunk for security:

```bash
# 1. Go to Settings → Integrations
# 2. Click "Add Integration"
# 3. Select "Splunk"
# 4. Enter:
   SPLUNK_URL=https://splunk.yourcompany.com:8089
   SPLUNK_API_TOKEN=xxxxxxxxxxxxx
   IMPORT_ALERTS=true
# 5. Click "Test Connection"
# 6. Click "Enable"
```

Splunk alerts will auto-import to OpenRisk!

### TheHive Integration

If you use TheHive for incidents:

```bash
# Settings → Integrations → TheHive
   THEHIVE_URL=https://thehive.yourcompany.com
   THEHIVE_API_KEY=xxxxxxxxxxxxx
# Bi-directional sync enabled!
```

---

## 📝 Step 8: Invite Team Members (Optional)

### Add a Team Member

1. Go to "Settings" → "Team"
2. Click "Invite User"
3. Enter email: `john@yourcompany.com`
4. Select role:
   ```
   - Admin: Full access
   - Risk Manager: Create/edit risks
   - Analyst: View & comment
   - Viewer: Read-only
   ```
5. Click "Send Invite"

User receives invitation email!

---

## 🎯 Useful Commands

### Check Status

```bash
# Is everything running?
docker compose ps

# View logs
docker compose logs backend
docker compose logs frontend

# Restart services
docker compose restart
```

### Stop / Restart

```bash
# Stop
docker compose down

# Stop and remove data
docker compose down -v

# Restart
docker compose up -d
```

### Reset Test Data

```bash
# Clear and start fresh
docker compose down -v
docker compose up -d
# Then import data (Step 4)
```

---

## 🚨 Troubleshooting

### "Connection refused" on localhost:5173

```bash
# Frontend didn't start
# Solution:
docker compose restart frontend
docker compose logs frontend  # See error

# Or wait 30 seconds, Docker is slow on first start
```

### "Database connection error"

```bash
# Database not ready
# Solution:
docker compose logs db  # Check logs

# Or:
docker compose down -v
docker compose up -d
```

### "Can't login with admin@openrisk.local"

```bash
# Default credentials not working
# Solution:
# 1. Verify backend is running
docker compose ps | grep backend
# Should be "UP"

# 2. Check migrations applied
docker compose logs backend | grep "migration"

# 3. Full reset
docker compose down -v
docker compose up -d
# Wait 30 seconds

# 4. Try again
```

### Port 5173 already in use

```bash
# Another process using the port
# Solution:

# Option 1: Find the process
lsof -i :5173
kill -9 <PID>

# Option 2: Use different port
docker compose down
# Edit docker-compose.yaml, frontend section:
#   ports:
#     - "5174:5173"  # ← Change 5173 to 5174
docker compose up -d

# Access http://localhost:5174
```

---

## 📚 Next Steps

### Go Deeper

1. **Read real use cases**: [USE_CASES_EN.md](USE_CASES_EN.md)
2. **Explore full API**: [API_REFERENCE.md](API_REFERENCE.md)
3. **Configure SSO**: [SAML_OAUTH2_INTEGRATION.md](SAML_OAUTH2_INTEGRATION.md)
4. **Deploy to Production**: [PRODUCTION_RUNBOOK.md](PRODUCTION_RUNBOOK.md)
5. **Integrate your tools**: [SYNC_ENGINE.md](SYNC_ENGINE.md)

### Recommended Reading

| Doc | For | Time |
|-----|-----|------|
| [USE_CASES_EN.md](USE_CASES_EN.md) | Discover real value | 5 min |
| [API_REFERENCE.md](API_REFERENCE.md) | Developers & API | 10 min |
| [SAML_OAUTH2_INTEGRATION.md](SAML_OAUTH2_INTEGRATION.md) | IT & Admins | 15 min |
| [LOCAL_DEVELOPMENT.md](LOCAL_DEVELOPMENT.md) | Contributing | 20 min |

---

## ❓ Questions?

- 💬 **Chat**: [GitHub Discussions](https://github.com/alex-dembele/OpenRisk/discussions)
- 🐛 **Bug**: [Open an Issue](https://github.com/alex-dembele/OpenRisk/issues)
- 📖 **Docs**: [See all guides](./README.md)

---

## 🎉 Congratulations!

You just deployed a **complete risk management platform** in 5 minutes!

**Next?** → Read [USE_CASES_EN.md](USE_CASES_EN.md) to see how to use it for your team 🚀
