# OpenRisk - Real-World Use Cases

This document presents 3 concrete use cases where OpenRisk creates immediate value.

---

## 📌 Use Case 1: SaaS Startup - Measure & Prioritize Production Risks

### The Problem
**TechStart.io** is a SaaS startup with 50 employees and 2000 customers. Their risk management process is manual:
- Risks documented in Google Sheets
- No centralized scoring
- Security alerts accumulate without prioritization
- CISO works 70 hours/week tracking manually

### Solution with OpenRisk

#### 1️⃣ Initial Setup (30 min)
```bash
# Start OpenRisk
docker compose up -d

# Access the interface
# → http://localhost:5173
# Email: admin@opendefender.io | Password: see docs/QUICK_ONBOARDING_EN.md (there is no default)
```

#### 2️⃣ Create Risk Categories
From the interface:
- **Infrastructure** (servers, databases, networks)
- **Application** (bugs, software vulnerabilities)
- **Data** (leaks, GDPR compliance)
- **Operations** (incidents, RTO/RPO)

#### 3️⃣ Assess Existing Risks
Example: **Vulnerability in Node.js v18**

```
Title: Node.js 18 Vulnerability - HTTP Injection
Description: An attacker can send malicious headers
Framework: OWASP Top 10 - Injection
Criticality: High (Availability)
Probability: Medium (requires exploitation)

Automatic Score: 7.2/10 (High Priority)
```

#### 4️⃣ Create Mitigation Plan
```
Mitigation: Upgrade Node.js 18 → 20 LTS
Status: In Progress
Owner: DevOps Lead
Deadline: January 15, 2026

Sub-actions (Checklist):
☑️ Test on staging environment
☑️ Validate dependencies
☐ Deploy to production
☐ Monitor for 48 hours after deployment
```

#### 5️⃣ Real-Time Dashboard
The CISO sees at a glance:
- **8 High risks** → Require immediate action
- **12 Medium risks** → Need planning
- **5 Low risks** → Monitor
- **Trend chart** → Shows 3 risks resolved this month

### 💡 Real Impact
| Before | After |
|--------|-------|
| 70h/week manual management | 5h/week follow-up |
| No visibility for exec team | Real-time dashboard |
| Risks forgotten | 100% tracked |
| Monthly reports = emergency | Reports generated in 2 clicks |

**Result**: The CISO can focus on strategy instead of administration.

---

## 📌 Use Case 2: SME - Centralize Security Alerts

### The Problem
**SecureLogistics.fr** is an SME with 150 employees and hybrid infrastructure:
- On-premise servers + AWS
- Elastic Stack for logs
- Splunk for security
- Alerts arrive everywhere: email, Slack, Jira tickets
- Impossible to track "who needs to do what"

### Solution with OpenRisk

#### 1️⃣ Import Existing Data
OpenRisk can connect to your existing tools:

```bash
# Configuration in interface (Settings → Integrations)

# Option 1: Splunk Integration
API_SPLUNK_URL=https://splunk.securelog.fr:8089
API_SPLUNK_TOKEN=xxxxx
IMPORT_ALERTS=true

# Option 2: Elastic Integration  
ELASTICSEARCH_URL=https://elastic.securelog.fr:9200
IMPORT_ALERTS=true

# Option 3: Manual (import CSV)
# Upload your file in OpenRisk
```

#### 2️⃣ Example: Splunk Alert "SSH Brute-Force Attack"

**Alert arrives:**
```
[CRITICAL] 47 failed SSH attempts on srv-prod-01
Source: 203.0.113.45
Time: 2025-12-22 14:32:00
```

**In OpenRisk:**
- Create Risk: "SSH brute-force attack"
- Auto-score: 8.5/10 (Criteria: repeated attempts + production)
- Assign to: Infrastructure Owner
- Link to Mitigation: "Implement fail2ban"
- Sub-actions:
  ```
  ☑️ Block IP immediately
  ☐ Check if access granted
  ☐ Implement rate limiting
  ☐ Require 2FA mandatory
  ```

#### 3️⃣ Centralized Dashboard
One place to see:
- 🔴 **Critical active**: 3
- 🟠 **High**: 7
- 🟡 **Medium**: 15
- 🟢 **Low**: 32
- **Chart**: Trend over last 30 days

#### 4️⃣ Team Integration
```
Slack Integration:
- Notification when new Critical risk
- Daily digest of 5 risks to handle
- Weekly report
```

### 💡 Real Impact
| Before | After |
|--------|-------|
| Scattered alerts = many forgotten | 100% centralized |
| 3-4h searching "where is the alert" | 30s to find information |
| No prioritization order | Automatic score sorting |
| Blurry responsibility | Each risk has an owner |

**Result**: Alerts become tracked actions, not noise.

---

## 📌 Use Case 3: CISO - Automated Quarterly Reports

### The Problem
**MegatechCorp.com** is a large enterprise with 500 employees. The CISO must:
- Produce compliance report **every quarter**
- Show identified risks
- Prove mitigations are progressing
- Submit to board + external auditors
- Currently: **5 days of work** per report

### Solution with OpenRisk

#### 1️⃣ Annual Setup (1 hour)

```bash
# In Settings → Organization
Compliance_Framework: ISO 27001
Report_Frequency: Quarterly
Auto_Export_Format: PDF + Excel
Recipients: 
  - direction@megatech.fr
  - audit@megatech.fr
  - ciso@megatech.fr
```

#### 2️⃣ Example: Q4 2025 Report

**OpenRisk generates automatically:**

```
📊 QUARTERLY RISK MANAGEMENT REPORT
Period: Oct - Dec 2025
Generated: December 22, 2025

1. EXECUTIVE SUMMARY
   ✅ 47 risks identified
   ✅ 12 risks resolved this quarter (-20%)
   ✅ 8 mitigations in progress (deadline: Q1 2026)
   ⚠️  3 Critical risks escalated to Board

2. TRENDS
   [Chart] Risk count evolution
   - Trend: ↓ -15% vs Q3 (Positive!)
   - Resolutions: 12 risks
   - New: 8 risks

3. DETAIL BY DOMAIN
   
   Infrastructure: 15 risks
   ├─ Critical: 1 (Old Windows XP server)
   ├─ High: 3
   └─ Medium: 11

   Application: 18 risks
   ├─ Critical: 2 (Outdated dependencies)
   ├─ High: 5
   └─ Medium: 11

   Data & Compliance: 14 risks
   ├─ Critical: 0
   ├─ High: 4
   └─ Medium: 10

4. MITIGATIONS IN PROGRESS
   
   ✅ Node.js Upgrade (70% complete)
      └─ Deadline: Jan 15, 2026
   
   ✅ Implement MFA (50% complete)
      └─ Deadline: Feb 28, 2026
   
   ✅ External security audit (30% complete)
      └─ Deadline: Mar 31, 2026

5. COMPLIANCE STATUS
   ISO 27001: ✅ 92% covered (vs 85% Q3)
   GDPR: ✅ 100% covered
   SOC2: ✅ 88% in progress

6. RECOMMENDATIONS
   - Accelerate Node.js upgrade (Critical)
   - Implement MFA immediately (Security)
   - Refactor legacy architecture (Medium term)

---
Digitally signed by OpenRisk v1.0.4
```

#### 2️⃣ Export the Report

**From OpenRisk:**
```bash
# Interface: Reports → Download Quarterly Report
# Available formats:
# - PDF (ready to print)
# - Excel (for analysis)
# - JSON (for BI tools)
```

#### 3️⃣ Time Required

**Before**: 5 days (manual collection + formatting)
```
Day 1: Send emails to teams
Day 2-3: Collect responses
Day 4: Format in PowerPoint
Day 5: Validation + corrections
```

**With OpenRisk**: 10 minutes
```
1. Click "Generate Quarterly Report"
2. Download PDF
3. Send to stakeholders
```

### 💡 Real Impact
| Before | After |
|--------|-------|
| 5 days/month preparation | 30 min/quarter |
| Potentially outdated data | Real-time data |
| Impossible to track evolution | Trend charts |
| Format varies each time | Consistent & professional |

**Result**: The CISO can justify the budget to the board with precise data.

---

## 🎯 Summary: Why OpenRisk?

### For Startups
✅ Automate = less manual time  
✅ Prioritize = focus on what matters  
✅ Scale = easily go from 10 to 1000 risks

### For SMEs
✅ Centralize = single source of truth  
✅ Integrate = connect existing tools  
✅ Report = prove security

### For Enterprises
✅ Automate = save 100+ days/year per CISO  
✅ Audit = compliance reports in 10 min  
✅ Govern = complete visibility for board

---

## 📞 Ready to try?

**[→ Get Started in 5 Minutes](QUICK_ONBOARDING.md)**

Questions? Check [API_REFERENCE.md](API_REFERENCE.md) or open a [discussion](https://github.com/alex-dembele/OpenRisk/discussions).
