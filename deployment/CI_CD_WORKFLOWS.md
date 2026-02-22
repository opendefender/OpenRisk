# GitHub Actions CI/CD Workflows

**Date**: February 22, 2026  
**Status**: Production Ready

---

## Overview

This document describes the comprehensive GitHub Actions CI/CD pipeline for OpenRisk, ensuring code quality, security, and automated deployment to staging and production environments.

### Workflow Architecture

```
Pull Request → CI Tests → Code Quality Checks → Build Artifacts
                ↓
            Success → Merge to develop/stag/main
                ↓
            stag branch → Deploy to Staging
            main branch → Deploy to Production
```

---

## Workflows Overview

### 1. **CI Pipeline** (`.github/workflows/ci.yml`)

Runs on every push and pull request to verify code quality and build artifacts.

#### Jobs:

| Job | Purpose | Triggers | Time |
|-----|---------|----------|------|
| `backend-lint` | Go code style, format, and import checks | PR, push | 2-3 min |
| `backend-unit-tests` | Unit test execution with coverage | PR, push | 3-4 min |
| `backend-integration-tests` | Database and service integration tests | PR, push | 5-7 min |
| `frontend-lint` | ESLint, TypeScript, Prettier checks | PR, push | 2-3 min |
| `frontend-unit-tests` | Jest tests with coverage reporting | PR, push | 3-5 min |
| `build-backend` | Go binary compilation | Success | 2-3 min |
| `build-frontend` | React/Vite build process | Success | 3-4 min |
| `build-docker-image` | Docker image build & push to registry | Success | 5-10 min |
| `ci-summary` | Final CI status summary | Always | <1 min |

**Total Duration**: ~15-20 minutes

#### Coverage Requirements

- **Backend**: 50% minimum code coverage (enforced)
- **Frontend**: 70% minimum code coverage (enforced)
- Uploaded to Codecov for tracking

### 2. **Security Workflow** (`.github/workflows/security.yml`)

Comprehensive security scanning on push and daily schedule.

#### Jobs:

| Job | Tool | Purpose | Frequency |
|-----|------|---------|-----------|
| `dependency-check` | Trivy | Scan dependencies for CVEs | Push + Daily |
| `golang-security` | gosec | Static Go security analysis | Push + Daily |
| `container-scan` | Trivy | Docker image vulnerability scan | Push + Daily |
| `code-quality` | gofmt, staticcheck | Code style and quality | Push |
| `sast-semgrep` | Semgrep | SAST for OWASP Top 10 | Push + Daily |
| `dast-owasp-zap` | OWASP ZAP | Dynamic security testing | Daily |
| `license-check` | Go mod | License compliance verification | Push |

**Triggers**:
- Every push to `main`, `stag`, `develop`
- Daily at 2 AM UTC
- Can be run manually via workflow dispatch

### 3. **E2E Tests** (`.github/workflows/e2e.yml`)

End-to-end testing across multiple browsers with performance benchmarks.

#### Jobs:

| Job | Purpose | Browsers | Time |
|-----|---------|----------|------|
| `e2e-tests` | Playwright E2E tests | Chromium, Firefox, WebKit | 10-15 min each |
| `performance-tests` | k6 load testing | Single run | 3-5 min |

**Triggers**: Push to `main`, `stag`, `develop` and PRs

**Test Scenarios**:
- User authentication flow
- Risk management operations
- Mitigation tracking
- Dashboard rendering
- API integration tests
- Mobile responsive behavior

### 4. **Deployment** (`.github/workflows/deploy.yml`)

Automated deployment to staging and production with approval gates.

#### Jobs:

| Job | Environment | Condition | Actions |
|-----|-------------|-----------|---------|
| `build-image` | Both | Success | Build Docker image, push to registry |
| `deploy-staging` | Staging | Push to `stag` | Deploy, run smoke tests |
| `deploy-production` | Production | Push to `main` or manual trigger | Backup, deploy, verify, monitor |

**Deployment Flow**:
- Staging: Auto-deploy on `stag` branch push
- Production: Manual trigger required or auto on `main` push
- Rollback on failure: Automatic
- Monitoring: Post-deployment metrics check

---

## Branch Strategy

### Branch Protection Rules

```
main (Production)
├─ Requires PR review: 1 approval
├─ Requires status checks to pass
├─ Requires branches to be up to date
└─ Auto-merge on approval

stag (Staging)
├─ Requires status checks to pass
└─ Auto-deploy on push

develop (Development)
├─ All PRs merged here first
└─ No deployment
```

### Branch Workflow

```
feature/* → develop → PR review → stag → staging deployment
                         ↓
                      main → production deployment
```

---

## GitHub Actions Secrets Configuration

Required secrets in `.github/settings`:

```yaml
GITHUB_TOKEN              # Auto-provided
CODECOV_TOKEN            # Codecov.io upload
KUBE_CONFIG_STAGING      # Kubernetes staging credentials
KUBE_CONFIG_PRODUCTION   # Kubernetes production credentials
DATABASE_URL_TEST        # Test database connection
SLACK_WEBHOOK_URL        # Slack notifications
PAGERDUTY_SERVICE_KEY    # PagerDuty integration
OPSGENIE_API_KEY         # Opsgenie integration
```

### Setup Secrets

1. Go to repository **Settings** → **Secrets and variables** → **Actions**
2. Create each secret with appropriate value
3. Secrets are encrypted and not logged in workflow output

---

## Workflow Triggers

### CI Pipeline Triggers

**On**: 
- Push to `main`, `stag`, `develop`
- Pull request to `main`, `stag`, `develop`

**Skips if**:
- Commit message contains `[skip ci]`
- Only documentation changes

### Security Workflow Triggers

**On**:
- Push to `main`, `stag`, `develop`
- Pull request to `main`, `stag`, `develop`
- Daily schedule: 2 AM UTC
- Manual trigger available

### Deployment Workflow Triggers

**On**:
- Push to `stag` → Deploy to staging
- Push to `main` → Deploy to production (requires approval)
- Manual trigger with environment selection

---

## Performance Metrics

### CI Pipeline Performance

| Component | Duration | Target |
|-----------|----------|--------|
| Linting | 2-3 min | < 3 min ✅ |
| Unit Tests | 3-4 min | < 5 min ✅ |
| Integration Tests | 5-7 min | < 8 min ✅ |
| Security Scans | 5-10 min | < 15 min ✅ |
| Docker Build | 5-10 min | < 15 min ✅ |
| **Total** | **~20 min** | **< 25 min** ✅ |

### Optimization Strategies

1. **Parallel Jobs**: Unrelated jobs run simultaneously
2. **Docker Layer Caching**: Speeds up builds by 50%
3. **Dependency Caching**: npm and Go mod cached
4. **Matrix Testing**: Multiple browsers run in parallel
5. **Artifact Upload**: Only essential artifacts uploaded

---

## Test Coverage & Quality Gates

### Code Coverage Requirements

- **Backend (Go)**:
  - Minimum: 50%
  - Target: 70%
  - Critical paths: 90%

- **Frontend (TypeScript/React)**:
  - Minimum: 70%
  - Target: 80%
  - Critical components: 95%

### Coverage Tracking

- Codecov integration for historical tracking
- Coverage badges in README
- Coverage reports as PR comments
- Trend analysis dashboard

---

## Security Scanning Details

### Dependency Scanning

**Tool**: Trivy

```
Scans for:
├─ Known CVEs in dependencies
├─ License compliance
├─ License restrictions
└─ Outdated versions
```

### SAST - Static Application Security Testing

**Tools**:
- **Semgrep**: OWASP Top 10, CWE top 25, language-specific rules
- **gosec**: Go-specific security issues
- **staticcheck**: Code correctness issues

### DAST - Dynamic Application Security Testing

**Tool**: OWASP ZAP

```
Tests for:
├─ SQL injection
├─ Cross-site scripting (XSS)
├─ Cross-site request forgery (CSRF)
├─ Broken authentication
├─ Sensitive data exposure
├─ XML external entities (XXE)
├─ Broken access control
├─ Security misconfiguration
├─ Insecure deserialization
└─ Vulnerable libraries
```

---

## Deployment Procedures

### Staging Deployment

**Trigger**: Push to `stag` branch

**Steps**:
1. Build Docker image with `stag-` prefix
2. Push to container registry
3. Deploy to staging Kubernetes cluster
4. Run smoke tests
5. Monitor metrics for 5 minutes
6. Notify team on Slack

**Time**: ~5-10 minutes

**Rollback**: Manual or automatic on test failure

### Production Deployment

**Trigger**: Push to `main` branch (or manual dispatch)

**Prerequisites**:
- All CI tests passing
- Code reviewed and approved
- Staging validation complete

**Steps**:
1. Create database backup
2. Build Docker image with production tag
3. Push to container registry
4. Deploy to production cluster
5. Verify deployment health
6. Run smoke tests
7. Monitor metrics
8. Create deployment log

**Time**: ~10-15 minutes

**Safety**:
- No concurrent deployments (concurrency lock)
- Automatic rollback on health check failure
- Full deployment logs for audit trail

### Rollback Procedure

**Manual Rollback**:
```bash
kubectl rollout undo deployment/openrisk-backend -n production
kubectl rollout undo deployment/openrisk-frontend -n production
```

**Automatic Rollback** (on failure):
- Triggered if health checks fail
- Reverts to previous stable image
- Sends alert notification
- Creates incident issue

---

## Monitoring & Alerts

### GitHub Actions Monitoring

**Available Dashboards**:
- GitHub Actions tab → All workflows
- Branch-specific workflow runs
- Per-commit status indicators

### Notifications

**Slack Integration**:
```
#ci-notifications      - All CI job results
#deployment-alerts     - Deployment status
#security-alerts       - Security scan findings
#performance           - Performance test results
```

**PagerDuty**:
- Critical security findings trigger P1 incidents
- Production deployment failures trigger P2 incidents

---

## Troubleshooting

### Failed CI Pipeline

**Check**:
1. GitHub Actions tab for job logs
2. Specific job failure reason
3. Recent code changes

**Common Issues**:

| Issue | Cause | Solution |
|-------|-------|----------|
| Lint failure | Code format | Run `make fmt` locally |
| Test failure | Code bug | Review test logs, fix code |
| Coverage below threshold | Insufficient tests | Add test cases |
| Timeout | Slow test | Optimize or increase timeout |
| Docker build fails | Missing file | Check Dockerfile context |

### Rerun Failed Jobs

```bash
# Via GitHub UI:
# 1. Go to Actions tab
# 2. Select workflow
# 3. Click "Re-run all jobs" or "Re-run failed jobs"

# Via GitHub CLI:
gh run rerun <run-id>
```

### Debug Workflow Locally

```bash
# Install act (local GitHub Actions runner)
brew install act

# Run workflow locally
act push -j backend-unit-tests
```

---

## Best Practices

### 1. Commit Messages

```
feat: Add new feature
fix: Fix bug
chore: Update dependencies
[skip ci] Documentation update  # Skip CI for doc changes
```

### 2. PR Requirements

- Clear description of changes
- Link to related issues
- All CI checks passing
- Code review approved
- Test coverage maintained

### 3. Deployment Checklist

Before production deployment:
- [ ] All tests passing on CI
- [ ] Staging validation complete
- [ ] Monitoring dashboards accessible
- [ ] Team notified of deployment window
- [ ] Rollback plan documented
- [ ] On-call engineer available

### 4. Performance Optimization

- Keep jobs under 10 minutes
- Use caching for dependencies
- Parallelize independent jobs
- Remove unnecessary artifacts
- Monitor workflow execution time

---

## GitHub Actions Limits

### Usage Limits

- **Public repositories**: Unlimited minutes
- **Private repositories**: 2,000 minutes/month (included)
- **Concurrent jobs**: 20 (per account)
- **Job timeout**: 6 hours (default 35 min)

### Storage Limits

- **Artifact retention**: 30 days (configurable)
- **Artifact size**: 5 GB per run
- **Log retention**: 90 days

---

## Maintenance & Updates

### Regular Tasks

- **Weekly**: Review workflow run times and failures
- **Monthly**: Update action versions to latest
- **Quarterly**: Review and optimize workflows
- **Yearly**: Security audit of secrets and permissions

### Updating Go and Node Versions

```yaml
# In workflow files, update version numbers:
- uses: actions/setup-go@v4
  with:
    go-version: '1.25'  # Update as needed

- uses: actions/setup-node@v3
  with:
    node-version: '18'  # Update as needed
```

---

## Integration with External Services

### Codecov

1. Visit codecov.io
2. Connect GitHub repository
3. Configure coverage upload
4. Badge added to README

### Container Registry

Configured to push to: `ghcr.io/opendefender/OpenRisk`

**Images**:
- `ghcr.io/opendefender/OpenRisk:main` (latest prod)
- `ghcr.io/opendefender/OpenRisk:stag` (latest staging)
- `ghcr.io/opendefender/OpenRisk:sha-<commit>` (specific commit)

---

## Related Documentation

- [CI/CD Pipeline Diagram](./workflows/README.md)
- [Deployment Guide](../DEPLOYMENT_GUIDE.md)
- [Testing Guide](../../docs/TESTING_GUIDE.md)
- [Security Policy](../../SECURITY.md)

---

## FAQ

**Q: How long does a full CI run take?**
A: Approximately 20 minutes (with parallel jobs).

**Q: Can I skip CI for a commit?**
A: Yes, add `[skip ci]` to commit message (not recommended for production).

**Q: How do I trigger deployment manually?**
A: Go to Actions → Deploy workflow → Run workflow → Select environment.

**Q: What if deployment fails?**
A: Automatic rollback to previous version; check logs and fix issue.

**Q: How are secrets kept secure?**
A: Encrypted at rest, masked in logs, only accessible to authorized actors.

---

**Last Updated**: February 22, 2026  
**Next Review**: March 22, 2026  
**Status**: Production Ready
