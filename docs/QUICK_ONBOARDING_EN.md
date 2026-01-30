 OpenRisk - Quick Start Guide

Welcome! This guide gets you up and running in  minutes with realistic sample data.

---

  Step : Start the System ( min)

 Prerequisites
- Docker & Docker Compose installed
- Git
- A terminal (Bash, Zsh, PowerShell, etc.)

 Launch OpenRisk

bash
 . Clone the repository
git clone https://github.com/alex-dembele/OpenRisk.git
cd OpenRisk

 . Start all services
docker compose up -d

 . Verify everything is running
docker compose ps
 Should show: db, redis, backend, frontend (all UP)

 . Access the interface
 → Frontend: http://localhost:
 → API Backend: http://localhost:


  Health Check

bash
 Verify services are responding
curl http://localhost:/health
 Expected result: {"status":"healthy"}


---

  Step : Login ( min)

 Default Credentials

 Email: admin@openrisk.local
 Password: admin


 First Login

. Open http://localhost: in your browser
. Enter the credentials above
. Click "Login"

You're now on the Dashboard!

---

  Step : Explore the Dashboard ( sec)

You'll see  sections:

  Top Left: Overview

 High Risks
 Medium Risks
 Low Risks


  Top Right: Trend Chart

Shows risk evolution over the last  days
(Currently empty, we'll add data next)


  Bottom Left: Heatmap

Probability vs Impact matrix
Visualize risks visually


  Bottom Right: Recent Risks

List of recently created risks
(Currently empty)


---

  Step : Import Test Data ( min)

 Option A: Import via API (Recommended)

Download test file:

bash
 File is included in the repo
cat dev/fixtures/risks.json


Import the data:

bash
 Option : Via cURL (command line)
curl -X POST http://localhost:/api/risks/bulk-import \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d @dev/fixtures/risks.json

 Option : Via interface (easier)
 . Go to Settings → Data Management
 . Click "Import Data"
 . Upload dev/fixtures/risks.json
 . Click "Import"


 Option B: Create a Risk Manually

. Click "Risks" in the menu
. Click "Create Risk"
. Fill the form:


Title: SQL Injection vulnerability in login form
Description: User input is not escaped
Framework: OWASP Top  - A: Injection
Criticality: High
Probability: Medium
Status: Identified

Auto-calculated Score: ./ 


. Click "Save"

---

  Step : Create a Mitigation ( min)

 From an Existing Risk

. Click a risk (e.g., "SQL Injection")
. Go to "Mitigations" tab
. Click "Add Mitigation"
. Fill:


Title: Use Prepared Statements
Description: Refactor database layer
Status: In Progress
Owner: Backend Team Lead
Deadline: January , 


 Add Sub-Actions (Checklist)


Sub-actions:
 Validate with security team
 Write unit tests
 Deploy to staging
 Test for  hours in production
 Monitor logs


Check as you go:
bash
 When action is done, click  → 
 System auto-tracks progress


---

  Step : Generate a Report ( min)

 Create a Simple Report

. Click "Reports" in menu
. Click "Create Report"
. Select:
   - Type: Risk Summary
   - Period: This Month
   - Format: PDF
. Click "Generate"

Report generated in  seconds!

 What's in the Report


 RISK MANAGEMENT REPORT
Generated: December , 

Summary:
- Total risks: 
- Critical: 
- High: 
- Medium: 

Details:
. SQL Injection (Score: .) → Mitigation in progress
. ...

Recommended Actions:
- Accelerate Critical mitigation
- ...


---

  Step : Connect Your Tools (Optional)

 Splunk Integration

If you use Splunk for security:

bash
 . Go to Settings → Integrations
 . Click "Add Integration"
 . Select "Splunk"
 . Enter:
   SPLUNK_URL=https://splunk.yourcompany.com:
   SPLUNK_API_TOKEN=xxxxxxxxxxxxx
   IMPORT_ALERTS=true
 . Click "Test Connection"
 . Click "Enable"


Splunk alerts will auto-import to OpenRisk!

 TheHive Integration

If you use TheHive for incidents:

bash
 Settings → Integrations → TheHive
   THEHIVE_URL=https://thehive.yourcompany.com
   THEHIVE_API_KEY=xxxxxxxxxxxxx
 Bi-directional sync enabled!


---

  Step : Invite Team Members (Optional)

 Add a Team Member

. Go to "Settings" → "Team"
. Click "Invite User"
. Enter email: john@yourcompany.com
. Select role:
   
   - Admin: Full access
   - Risk Manager: Create/edit risks
   - Analyst: View & comment
   - Viewer: Read-only
   
. Click "Send Invite"

User receives invitation email!

---

  Useful Commands

 Check Status

bash
 Is everything running?
docker compose ps

 View logs
docker compose logs backend
docker compose logs frontend

 Restart services
docker compose restart


 Stop / Restart

bash
 Stop
docker compose down

 Stop and remove data
docker compose down -v

 Restart
docker compose up -d


 Reset Test Data

bash
 Clear and start fresh
docker compose down -v
docker compose up -d
 Then import data (Step )


---

  Troubleshooting

 "Connection refused" on localhost:

bash
 Frontend didn't start
 Solution:
docker compose restart frontend
docker compose logs frontend   See error

 Or wait  seconds, Docker is slow on first start


 "Database connection error"

bash
 Database not ready
 Solution:
docker compose logs db   Check logs

 Or:
docker compose down -v
docker compose up -d


 "Can't login with admin@openrisk.local"

bash
 Default credentials not working
 Solution:
 . Verify backend is running
docker compose ps | grep backend
 Should be "UP"

 . Check migrations applied
docker compose logs backend | grep "migration"

 . Full reset
docker compose down -v
docker compose up -d
 Wait  seconds

 . Try again


 Port  already in use

bash
 Another process using the port
 Solution:

 Option : Find the process
lsof -i :
kill - <PID>

 Option : Use different port
docker compose down
 Edit docker-compose.yaml, frontend section:
   ports:
     - ":"   ← Change  to 
docker compose up -d

 Access http://localhost:


---

  Next Steps

 Go Deeper

. Read real use cases: [USE_CASES_EN.md](USE_CASES_EN.md)
. Explore full API: [API_REFERENCE.md](API_REFERENCE.md)
. Configure SSO: [SAML_OAUTH_INTEGRATION.md](SAML_OAUTH_INTEGRATION.md)
. Deploy to Production: [PRODUCTION_RUNBOOK.md](PRODUCTION_RUNBOOK.md)
. Integrate your tools: [SYNC_ENGINE.md](SYNC_ENGINE.md)

 Recommended Reading

| Doc | For | Time |
|-----|-----|------|
| [USE_CASES_EN.md](USE_CASES_EN.md) | Discover real value |  min |
| [API_REFERENCE.md](API_REFERENCE.md) | Developers & API |  min |
| [SAML_OAUTH_INTEGRATION.md](SAML_OAUTH_INTEGRATION.md) | IT & Admins |  min |
| [LOCAL_DEVELOPMENT.md](LOCAL_DEVELOPMENT.md) | Contributing |  min |

---

  Questions?

-  Chat: [GitHub Discussions](https://github.com/alex-dembele/OpenRisk/discussions)
-  Bug: [Open an Issue](https://github.com/alex-dembele/OpenRisk/issues)
-  Docs: [See all guides](./README.md)

---

  Congratulations!

You just deployed a complete risk management platform in  minutes!

Next? → Read [USE_CASES_EN.md](USE_CASES_EN.md) to see how to use it for your team 
