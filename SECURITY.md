# Security Policy

## Security Commitment

OpenRisk takes the security of our software products, services, and community seriously. We are committed to maintaining the highest standards of security to protect the data and privacy of our users.

If you believe you have found a security vulnerability in OpenRisk, please report it to us as described below. **Please do not open public GitHub issues for security vulnerabilities.**

---

## Reporting Security Issues

### How to Report

**Email**: [security@openrisk.io](mailto:security@openrisk.io)

**PGP Key**: [Download our PGP key](https://openrisk.io/.well-known/security.asc)

**CVSS Scoring**: Please include CVSS v3.1 Base Score if available

### What to Include

When reporting a security vulnerability, please provide:

1. **Description of the vulnerability**
   - What is the issue?
   - What's the impact?
   - How can it be exploited?

2. **Affected versions**
   - Which versions of OpenRisk are affected?
   - Is it in the latest version?

3. **Steps to reproduce**
   - Provide clear steps to reproduce the issue
   - Include proof-of-concept code if possible

4. **Your contact information**
   - Name and email address
   - Preferred communication method
   - Whether you want to be credited

5. **Timeline preferences**
   - Preferred disclosure timeline
   - Any public disclosure date you have in mind

### Example Report

```
Subject: [SECURITY] SQL Injection in Risk API

Description:
The /api/v1/risks endpoint is vulnerable to SQL injection through the "name" parameter.

Affected Versions:
- OpenRisk v1.0.0 through v1.2.3
- Not affected: v1.2.4+

Steps to Reproduce:
1. Send POST request to /api/v1/risks
2. Include payload: {"name": "'; DROP TABLE risks; --"}
3. Database tables are deleted

Impact:
- Complete data loss
- Service disruption
- Data integrity violation

Proof of Concept:
[Include code snippet or cURL command]

Timeline:
- Discovered: 2026-03-02
- Reported: 2026-03-02
- Proposed fix timeline: 2026-03-09
```

---

## Response Process

### Our Commitment

We commit to:

1. **Acknowledge receipt** within **24 hours**
2. **Provide initial assessment** within **3 business days**
3. **Keep you updated** at regular intervals
4. **Provide ETA for fix** within **5 business days**
5. **Release patch** as soon as possible
6. **Credit the reporter** (if desired) in security advisories

### Timeline

| Phase | Timeline | Action |
|-------|----------|--------|
| **Initial Response** | Within 24 hours | We acknowledge receipt of your report |
| **Triage** | Within 3 days | We assess severity and impact |
| **Analysis** | Within 7 days | We analyze the vulnerability thoroughly |
| **Fix Development** | Within 14 days | We develop and test a fix |
| **Pre-release Testing** | Within 21 days | We complete security testing |
| **Patch Release** | Within 30 days | We release a security patch |
| **Public Disclosure** | 30+ days after fix | We publish a security advisory |

Note: Critical vulnerabilities may have accelerated timelines.

### Severity Levels

#### Critical (CVSS 9.0-10.0)
- Remote code execution
- Complete data breach
- Service-wide compromise
- **Response time**: 4 hours
- **Fix release**: 24-48 hours

#### High (CVSS 7.0-8.9)
- Authentication bypass
- Significant data exposure
- Major functionality compromise
- **Response time**: 8 hours
- **Fix release**: 5-7 days

#### Medium (CVSS 4.0-6.9)
- Partial data exposure
- Authentication issues
- DoS possibilities
- **Response time**: 24 hours
- **Fix release**: 10-14 days

#### Low (CVSS 0.1-3.9)
- Limited impact
- Information disclosure
- Low-impact bugs
- **Response time**: 3-5 days
- **Fix release**: 30 days

---

## Security Best Practices

### For Developers

- **Keep dependencies updated** - Run `npm audit` and `go mod tidy` regularly
- **Use parameterized queries** - Always escape user input
- **Enable HTTPS/TLS** - Encrypt all data in transit
- **Implement authentication** - Use industry-standard methods
- **Validate input** - Never trust user input
- **Use secrets management** - Never hardcode credentials
- **Implement CORS properly** - Restrict cross-origin requests
- **Log security events** - Track suspicious activities
- **Review code changes** - Require peer review for all PRs

### For Operators

- **Keep OpenRisk updated** - Deploy patches promptly
- **Use strong credentials** - Enforce password policies
- **Enable 2FA** - Use multi-factor authentication
- **Monitor access logs** - Review audit logs regularly
- **Restrict network access** - Use firewalls and VPNs
- **Backup data** - Maintain regular backups
- **Update dependencies** - Keep all packages current
- **Use RBAC** - Implement role-based access control
- **Encrypt sensitive data** - Use encryption at rest

### For Users

- **Use strong passwords** - At least 16 characters, mixed case
- **Enable 2FA** - Use authenticator apps or hardware keys
- **Review API keys** - Rotate keys periodically
- **Check audit logs** - Monitor account activity
- **Report suspicious activity** - Contact support immediately
- **Keep software updated** - Apply patches when available

---

## Security Headers

OpenRisk implements industry-standard security headers:

```
Strict-Transport-Security: max-age=31536000; includeSubDomains
X-Content-Type-Options: nosniff
X-Frame-Options: DENY
X-XSS-Protection: 1; mode=block
Content-Security-Policy: default-src 'self'
Referrer-Policy: strict-origin-when-cross-origin
Permissions-Policy: geolocation=(), microphone=(), camera=()
```

---

## Security Testing

### Regular Testing

We perform:

- **Static code analysis** - Automated security scanning
- **Dependency scanning** - Vulnerability detection in packages
- **Dynamic testing** - Runtime security validation
- **Penetration testing** - Manual security testing
- **Compliance audits** - Industry standard reviews

### Third-Party Audits

We conduct regular third-party security audits. Reports are available upon request.

---

## Known Vulnerabilities

Check for known vulnerabilities in OpenRisk dependencies:

```bash
# Backend
go list -json -m all | nancy sleuth

# Frontend
npm audit
yarn audit
```

---

## Disclosure Policy

### Coordinated Disclosure

We follow a coordinated disclosure policy:

1. **Report privately** - Send report to security@openrisk.io
2. **We'll confirm receipt** - Response within 24 hours
3. **Work together** - Collaborate on fix development
4. **Agree on timeline** - Typically 30-90 days
5. **Simultaneous release** - Patch and advisory released together
6. **Credit given** - Public acknowledgment (if desired)

### Public Disclosure

After a fix is released, we will:

1. Publish a security advisory on GitHub
2. List the CVE if applicable
3. Provide upgrade instructions
4. Update our security page

### Timeline Examples

**Scenario 1: Simple Fix**
- Day 1: Report received
- Day 3: Fix developed and tested
- Day 5: Patch released
- Day 5: Public advisory published

**Scenario 2: Complex Fix**
- Day 1: Report received
- Day 7: Fix developed and tested
- Day 14: Extended testing completed
- Day 14: Patch released
- Day 14: Public advisory published

---

## Supported Versions

Only the latest version of OpenRisk receives security updates.

| Version | Release Date | End of Life | Status |
|---------|--------------|------------|--------|
| 1.4.x | 2026-03-02 | 2027-03-02 | Supported |
| 1.3.x | 2025-12-01 | 2026-09-01 | Supported |
| 1.2.x | 2025-09-01 | 2026-06-01 | Limited |
| 1.1.x | 2025-06-01 | 2026-03-01 | Limited |
| < 1.1 | - | Ended | Unsupported |

**Note**: We recommend always upgrading to the latest version.

---

## Security-Related Links

- **GitHub Security Advisory** - https://github.com/opendefender/OpenRisk/security/advisories
- **Dependencies** - https://github.com/opendefender/OpenRisk/network/dependencies
- **Security Policy** - This file
- **Code of Conduct** - [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md)

---

## Compliance & Standards

OpenRisk aims to comply with:

- **OWASP Top 10** - Web application security
- **NIST Cybersecurity Framework** - Risk management
- **ISO/IEC 27001** - Information security management
- **GDPR** - Data protection regulation
- **SOC 2** - Service organization compliance

---

## Security Contacts

### Primary Contact
- **Email**: [security@openrisk.io](mailto:security@openrisk.io)
- **Response time**: Within 24 hours

### Alternative Contacts
- **General inquiries**: [info@openrisk.io](mailto:info@openrisk.io)
- **Compliance**: [compliance@openrisk.io](mailto:compliance@openrisk.io)
- **Privacy**: [privacy@openrisk.io](mailto:privacy@openrisk.io)

---

## Resources

- **Security Advisories** - https://github.com/opendefender/OpenRisk/security/advisories
- **Dependency Scanning** - https://github.com/opendefender/OpenRisk/security/dependabot
- **Code Scanning** - https://github.com/opendefender/OpenRisk/security/code-scanning
- **Security Documentation** - https://docs.openrisk.io/security

---

## Frequently Asked Questions

### Q: Do you pay for security vulnerabilities?
**A:** We do not offer a bug bounty program at this time, but we do offer recognition and credit for responsible disclosures.

### Q: Can I publicly disclose the vulnerability?
**A:** Please wait for our fix to be released and coordinated disclosure to be complete. Typically 30 days after initial report.

### Q: What if I don't hear back?
**A:** If you don't receive a response within 48 hours, please email [conduct@openrisk.io](mailto:conduct@openrisk.io) to escalate.

### Q: Can you keep my identity confidential?
**A:** Yes. We will only disclose information you authorize us to share.

### Q: How are security patches released?
**A:** Security patches are released as minor version updates (e.g., 1.2.3 -> 1.2.4) with detailed release notes explaining the fix.

---

## Attribution

This security policy is based on industry best practices from:

- [GitHub Security Policy Template](https://github.com/github/advisory-database)
- [OWASP Vulnerability Disclosure](https://owasp.org/www-community/attacks/Vulnerability_Disclosure)
- [Coordinated Disclosure Guidelines](https://cheatsheetseries.owasp.org/cheatsheets/Vulnerability_Disclosure_Cheat_Sheet.html)

---

**Last Updated**: March 2, 2026  
**Version**: 1.0  
**Next Review**: June 2, 2026

**Thank you for helping keep OpenRisk secure!**
