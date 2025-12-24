<div align="center">
  <img src="https://raw.githubusercontent.com/opendefender/OpenRisk/master/OpenRisk.png" alt="OpenRisk Logo" width="200" height="200" />
  
  # OpenRisk
  
  **Enterprise-Grade Risk Management Platform**
  
  Part of the [OpenDefender](https://github.com/opendefender) Ecosystem
  
  [![GitHub license](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
  [![GitHub release](https://img.shields.io/badge/version-1.0.4-brightgreen.svg)](https://github.com/alex-dembele/OpenRisk/releases)
  [![Go version](https://img.shields.io/badge/go-1.25.4-blue.svg)](https://golang.org)
  [![React version](https://img.shields.io/badge/react-19.2.0-61dafb.svg)](https://react.dev)
</div>

---

## 🎯 Overview

**OpenRisk** is a modern, enterprise-grade **Risk Management Platform** that transforms how organizations identify, assess, mitigate, and monitor risks. Built with a scalable microservices architecture, OpenRisk enables teams to move beyond spreadsheets and legacy systems into a seamless, automated risk management experience.

### 🎯 What OpenRisk Enables

OpenRisk allows every organization to:
- ✅ **Identify** IT & security risks
- ✅ **Score & Prioritize** risks based on impact and probability
- ✅ **Track** mitigation plans and action items
- ✅ **Monitor** trends in real-time with interactive dashboards

### 💡 Designed For

- **CTO & CISO** - Strategic risk oversight and compliance
- **DevSecOps** - Integrated security in CI/CD pipelines
- **Security Analysts** - Risk assessment and investigation
- **Compliance Teams** - Audit trails and governance

### 📈 Key Advantages

- ⚡ **Automated Risk Assessment** - Reduce manual evaluation time
- 📊 **Interactive Dashboards** - Real-time risk visualization
- 🔌 **Native Integrations** - Elastic, Splunk, TheHive, OpenCTI, AWS
- 🐳 **Easy Deployment** - Docker & Kubernetes ready
- 🔐 **Enterprise Security** - RBAC, SSO, audit logging
- 📈 **Scalable Architecture** - Microservices-ready

### Key Capabilities
- 🎲 **Risk Assessment** - Comprehensive risk identification and scoring
- 🛡️ **Mitigation Tracking** - Monitor and track risk mitigations in real-time
- 📊 **Advanced Analytics** - Real-time dashboards and trend analysis
- 🔐 **Enterprise Security** - RBAC, audit logging, OAuth2/SAML2 SSO
- 🔌 **Integration Ready** - TheHive, OpenCTI, Splunk, Elastic connectors
- ⚙️ **Custom Fields** - Flexible schema for organizational needs
- 📈 **Gamification** - Engagement and incentive system

---

## 🚀 Quick Start (5 Minutes)

### Prerequisites
- Docker & Docker Compose
- Git
- 4GB RAM, 2GB disk space

### Local Development

```bash
# Clone the repository
git clone https://github.com/alex-dembele/OpenRisk.git
cd OpenRisk

# Start all services (PostgreSQL, Redis, Backend, Frontend)
docker compose up -d

# Access the application
# Frontend: http://localhost:5173
# Backend API: http://localhost:8080
# API Docs: http://localhost:8080/swagger
```

### Default Credentials
```
Email: admin@openrisk.local
Password: admin123
```

---

## 🛠 Technical Stack

### Backend
| Component | Technology | Version |
|-----------|-----------|---------|
| **Language** | Go | 1.25.4 |
| **Framework** | Fiber | v2.52 |
| **Database** | PostgreSQL | 16 |
| **ORM** | GORM | v1.31 |
| **Testing** | Testify | v1.11 |
| **Architecture** | CLEAN | Domain-Driven |

### Frontend
| Component | Technology | Version |
|-----------|-----------|---------|
| **Framework** | React | 19.2.0 |
| **State** | Zustand | 5.0.8 |
| **Styling** | Tailwind CSS | 3.4.0 |
| **Forms** | React Hook Form | 7.66 |
| **Routing** | React Router | 7.9.6 |
| **Charts** | Recharts | 3.5.0 |

### Infrastructure
| Component | Technology | Purpose |
|-----------|-----------|---------|
| **Containerization** | Docker | Application packaging |
| **Orchestration** | Kubernetes | Production deployment |
| **Charts** | Helm | K8s configuration |
| **CI/CD** | GitHub Actions | Automated testing & deployment |
| **Caching** | Redis | Session & cache layer |

---

## 📋 Features

### Phase 1: Core Risk Management ✅
- ✅ Risk CRUD operations (Create, Read, Update, Delete, List)
- ✅ Risk scoring engine with weighted calculations
- ✅ Mitigation tracking with checklist sub-actions
- ✅ Asset management and relationships
- ✅ Soft-delete support with audit trails

### Phase 2: Security & Authentication ✅
- ✅ JWT-based authentication
- ✅ API Token management (create, revoke, rotate)
- ✅ Role-Based Access Control (RBAC)
- ✅ Permission matrices (resource-level granularity)
- ✅ Comprehensive audit logging
- ✅ OAuth2/SAML2 SSO (Google, GitHub, Azure AD)

### Phase 3: Infrastructure & Deployment ✅
- ✅ Docker Compose local development
- ✅ GitHub Actions CI/CD pipeline
- ✅ Integration test suite
- ✅ Kubernetes Helm charts
- ✅ Staging & production runbooks

### Phase 4: Enterprise Features ✅
- ✅ Custom fields framework (5 types)
- ✅ Bulk operations with validation
- ✅ Risk timeline (audit trail)
- ✅ Advanced reporting & export

### Phase 5: Advanced Analytics ✅
- ✅ Analytics dashboard with real-time data
- ✅ Risk heatmaps and trend analysis
- ✅ Incident management system
- ✅ Threat tracking and mapping
- ✅ Gamification & engagement system

---

## 📚 Documentation

| Document | Purpose |
|----------|---------|
| [LOCAL_DEVELOPMENT.md](docs/LOCAL_DEVELOPMENT.md) | Setup guide for development environment |
| [API_REFERENCE.md](docs/API_REFERENCE.md) | Complete API endpoint documentation |
| [KUBERNETES_DEPLOYMENT.md](docs/KUBERNETES_DEPLOYMENT.md) | K8s deployment instructions |
| [PRODUCTION_RUNBOOK.md](docs/PRODUCTION_RUNBOOK.md) | Production operations guide |
| [INTEGRATION_TESTS.md](docs/INTEGRATION_TESTS.md) | Testing procedures |
| [SAML_OAUTH2_INTEGRATION.md](docs/SAML_OAUTH2_INTEGRATION.md) | SSO integration guide |
| [SYNC_ENGINE.md](docs/SYNC_ENGINE.md) | Integration sync documentation |

For more documentation, see the [docs](docs/) directory.

---

## 🚀 Deployment

### Local Development
```bash
docker compose up -d
```

### Staging Environment
```bash
# See docs/STAGING_DEPLOYMENT.md
./scripts/deploy-kubernetes.sh --environment staging
```

### Production Deployment
```bash
# See docs/PRODUCTION_RUNBOOK.md
helm install openrisk ./helm/openrisk \
  -f helm/values-prod.yaml \
  --namespace openrisk
```

---

## 🧪 Testing

```bash
# Run all tests
make test-all

# Backend unit tests
cd backend && go test ./...

# Frontend tests
cd frontend && npm test

# Integration tests
./scripts/run-integration-tests.sh
```

**Test Statistics**: 142+ tests passing ✅

---

## 📊 API Overview

OpenRisk provides a comprehensive REST API with 29+ endpoints:

### Core Endpoints
```
POST   /api/risks              - Create risk
GET    /api/risks              - List risks
GET    /api/risks/:id          - Get risk details
PATCH  /api/risks/:id          - Update risk
DELETE /api/risks/:id          - Delete risk

POST   /api/mitigations        - Create mitigation
GET    /api/mitigations        - List mitigations
PATCH  /api/mitigations/:id    - Update mitigation

POST   /api/mitigations/:id/sub-actions    - Add checklist item
PATCH  /api/mitigations/:id/sub-actions/:aid - Toggle completion
```

### Security & Auth
```
POST   /auth/login             - JWT authentication
POST   /auth/register          - User registration
POST   /auth/oauth2/:provider  - OAuth2 login
POST   /auth/saml/acs          - SAML assertion endpoint

GET    /api/tokens             - List API tokens
POST   /api/tokens             - Create new token
DELETE /api/tokens/:id         - Revoke token
```

### Analytics & Reporting
```
GET    /api/analytics/dashboard     - Dashboard metrics
GET    /api/analytics/trends        - Risk trends
GET    /api/reports                 - List reports
POST   /api/reports/export          - Export risks/mitigations
```

See [API_REFERENCE.md](docs/API_REFERENCE.md) for complete endpoint documentation with examples.

---

## 🔐 Security

OpenRisk implements enterprise-grade security:

- **Authentication**: JWT tokens with expiration
- **Authorization**: RBAC with permission matrices
- **Encryption**: SHA256 hashing for sensitive data
- **Audit**: Complete audit trail for all operations
- **SSO**: OAuth2 and SAML2 support
- **Rate Limiting**: API rate limiting middleware
- **Input Validation**: Request validation with Zod/validator

See [ADVANCED_PERMISSIONS.md](docs/ADVANCED_PERMISSIONS.md) for detailed security documentation.

---

## 🤝 Contributing

We welcome contributions from the community! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

### Development Workflow
1. Fork the repository
2. Create a feature branch (`git checkout -b feature/AmazingFeature`)
3. Commit your changes (`git commit -m 'Add AmazingFeature'`)
4. Push to the branch (`git push origin feature/AmazingFeature`)
5. Open a Pull Request

---

## 📝 License

OpenRisk is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

---

## 🙋 Support & Contact

- **GitHub Issues**: [Report bugs or request features](https://github.com/alex-dembele/OpenRisk/issues)
- **Discussions**: [Join community discussions](https://github.com/alex-dembele/OpenRisk/discussions)
- **Security**: See [SECURITY.md](SECURITY.md) for security vulnerability reporting

---

## 🌟 Roadmap

### Q1 2026
- [ ] Multi-tenant support
- [ ] Advanced RBAC enhancements
- [ ] Additional connector integrations
- [ ] Mobile application (React Native)

### Q2 2026
- [ ] Marketplace for plugins
- [ ] Advanced reporting (BI dashboards)
- [ ] Machine learning risk predictions
- [ ] API webhook support

### Q3 2026
- [ ] Enterprise audit compliance
- [ ] Advanced analytics engine
- [ ] Custom dashboard builder
- [ ] Workflow automation

---

## 👥 Credits

**OpenRisk** is developed and maintained by the [OpenDefender](https://github.com/opendefender) community.

---

## 📞 Questions?

- 📖 Check the [documentation](docs/)
- 🐛 Search existing [issues](https://github.com/alex-dembele/OpenRisk/issues)
- 💬 Ask in [discussions](https://github.com/alex-dembele/OpenRisk/discussions)

---

<div align="center">
  Made with ❤️ by OpenDefender Community
  
  [⭐ Star us on GitHub](https://github.com/alex-dembele/OpenRisk)
</div>
