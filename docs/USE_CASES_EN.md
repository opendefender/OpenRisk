 OpenRisk - Real-World Use Cases

This document presents  concrete use cases where OpenRisk creates immediate value.

---

  Use Case : SaaS Startup - Measure & Prioritize Production Risks

 The Problem
TechStart.io is a SaaS startup with  employees and  customers. Their risk management process is manual:
- Risks documented in Google Sheets
- No centralized scoring
- Security alerts accumulate without prioritization
- CISO works  hours/week tracking manually

 Solution with OpenRisk

 ⃣ Initial Setup ( min)
bash
 Start OpenRisk
docker compose up -d

 Access the interface
 → http://localhost:
 Email: admin@openrisk.local | Password: admin


 ⃣ Create Risk Categories
From the interface:
- Infrastructure (servers, databases, networks)
- Application (bugs, software vulnerabilities)
- Data (leaks, GDPR compliance)
- Operations (incidents, RTO/RPO)

 ⃣ Assess Existing Risks
Example: Vulnerability in Node.js v


Title: Node.js  Vulnerability - HTTP Injection
Description: An attacker can send malicious headers
Framework: OWASP Top  - Injection
Criticality: High (Availability)
Probability: Medium (requires exploitation)

Automatic Score: ./ (High Priority)


 ⃣ Create Mitigation Plan

Mitigation: Upgrade Node.js  →  LTS
Status: In Progress
Owner: DevOps Lead
Deadline: January , 

Sub-actions (Checklist):
 Test on staging environment
 Validate dependencies
 Deploy to production
 Monitor for  hours after deployment


 ⃣ Real-Time Dashboard
The CISO sees at a glance:
-  High risks → Require immediate action
-  Medium risks → Need planning
-  Low risks → Monitor
- Trend chart → Shows  risks resolved this month

  Real Impact
| Before | After |
|--------|-------|
| h/week manual management | h/week follow-up |
| No visibility for exec team | Real-time dashboard |
| Risks forgotten | % tracked |
| Monthly reports = emergency | Reports generated in  clicks |

Result: The CISO can focus on strategy instead of administration.

---

  Use Case : SME - Centralize Security Alerts

 The Problem
SecureLogistics.fr is an SME with  employees and hybrid infrastructure:
- On-premise servers + AWS
- Elastic Stack for logs
- Splunk for security
- Alerts arrive everywhere: email, Slack, Jira tickets
- Impossible to track "who needs to do what"

 Solution with OpenRisk

 ⃣ Import Existing Data
OpenRisk can connect to your existing tools:

bash
 Configuration in interface (Settings → Integrations)

 Option : Splunk Integration
API_SPLUNK_URL=https://splunk.securelog.fr:
API_SPLUNK_TOKEN=xxxxx
IMPORT_ALERTS=true

 Option : Elastic Integration  
ELASTICSEARCH_URL=https://elastic.securelog.fr:
IMPORT_ALERTS=true

 Option : Manual (import CSV)
 Upload your file in OpenRisk


 ⃣ Example: Splunk Alert "SSH Brute-Force Attack"

Alert arrives:

[CRITICAL]  failed SSH attempts on srv-prod-
Source: ...
Time: -- ::


In OpenRisk:
- Create Risk: "SSH brute-force attack"
- Auto-score: ./ (Criteria: repeated attempts + production)
- Assign to: Infrastructure Owner
- Link to Mitigation: "Implement failban"
- Sub-actions:
  
   Block IP immediately
   Check if access granted
   Implement rate limiting
   Require FA mandatory
  

 ⃣ Centralized Dashboard
One place to see:
-  Critical active: 
-  High: 
-  Medium: 
-  Low: 
- Chart: Trend over last  days

 ⃣ Team Integration

Slack Integration:
- Notification when new Critical risk
- Daily digest of  risks to handle
- Weekly report


  Real Impact
| Before | After |
|--------|-------|
| Scattered alerts = many forgotten | % centralized |
| -h searching "where is the alert" | s to find information |
| No prioritization order | Automatic score sorting |
| Blurry responsibility | Each risk has an owner |

Result: Alerts become tracked actions, not noise.

---

  Use Case : CISO - Automated Quarterly Reports

 The Problem
MegatechCorp.com is a large enterprise with  employees. The CISO must:
- Produce compliance report every quarter
- Show identified risks
- Prove mitigations are progressing
- Submit to board + external auditors
- Currently:  days of work per report

 Solution with OpenRisk

 ⃣ Annual Setup ( hour)

bash
 In Settings → Organization
Compliance_Framework: ISO 
Report_Frequency: Quarterly
Auto_Export_Format: PDF + Excel
Recipients: 
  - direction@megatech.fr
  - audit@megatech.fr
  - ciso@megatech.fr


 ⃣ Example: Q  Report

OpenRisk generates automatically:


 QUARTERLY RISK MANAGEMENT REPORT
Period: Oct - Dec 
Generated: December , 

. EXECUTIVE SUMMARY
     risks identified
     risks resolved this quarter (-%)
     mitigations in progress (deadline: Q )
      Critical risks escalated to Board

. TRENDS
   [Chart] Risk count evolution
   - Trend: ↓ -% vs Q (Positive!)
   - Resolutions:  risks
   - New:  risks

. DETAIL BY DOMAIN
   
   Infrastructure:  risks
    Critical:  (Old Windows XP server)
    High: 
    Medium: 

   Application:  risks
    Critical:  (Outdated dependencies)
    High: 
    Medium: 

   Data & Compliance:  risks
    Critical: 
    High: 
    Medium: 

. MITIGATIONS IN PROGRESS
   
    Node.js Upgrade (% complete)
       Deadline: Jan , 
   
    Implement MFA (% complete)
       Deadline: Feb , 
   
    External security audit (% complete)
       Deadline: Mar , 

. COMPLIANCE STATUS
   ISO :  % covered (vs % Q)
   GDPR:  % covered
   SOC:  % in progress

. RECOMMENDATIONS
   - Accelerate Node.js upgrade (Critical)
   - Implement MFA immediately (Security)
   - Refactor legacy architecture (Medium term)

---
Digitally signed by OpenRisk v..


 ⃣ Export the Report

From OpenRisk:
bash
 Interface: Reports → Download Quarterly Report
 Available formats:
 - PDF (ready to print)
 - Excel (for analysis)
 - JSON (for BI tools)


 ⃣ Time Required

Before:  days (manual collection + formatting)

Day : Send emails to teams
Day -: Collect responses
Day : Format in PowerPoint
Day : Validation + corrections


With OpenRisk:  minutes

. Click "Generate Quarterly Report"
. Download PDF
. Send to stakeholders


  Real Impact
| Before | After |
|--------|-------|
|  days/month preparation |  min/quarter |
| Potentially outdated data | Real-time data |
| Impossible to track evolution | Trend charts |
| Format varies each time | Consistent & professional |

Result: The CISO can justify the budget to the board with precise data.

---

  Summary: Why OpenRisk?

 For Startups
 Automate = less manual time  
 Prioritize = focus on what matters  
 Scale = easily go from  to  risks

 For SMEs
 Centralize = single source of truth  
 Integrate = connect existing tools  
 Report = prove security

 For Enterprises
 Automate = save + days/year per CISO  
 Audit = compliance reports in  min  
 Govern = complete visibility for board

---

  Ready to try?

[→ Get Started in  Minutes](QUICK_ONBOARDING.md)

Questions? Check [API_REFERENCE.md](API_REFERENCE.md) or open a [discussion](https://github.com/alex-dembele/OpenRisk/discussions).
