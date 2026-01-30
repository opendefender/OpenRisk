<div align="center">
  <img src="https://raw.githubusercontent.com/opendefender/OpenRisk/master/OpenRisk.png" alt="OpenRisk Logo" width="" height="" />
  
   OpenRisk
  
  Enterprise-Grade Risk Management Platform
  
  Part of the [OpenDefender](https://github.com/opendefender) Ecosystem
  
  [![GitHub license](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
  [![GitHub release](https://img.shields.io/badge/version-..-brightgreen.svg)](https://github.com/opendefender/OpenRisk/releases)
  [![Go version](https://img.shields.io/badge/go-..-blue.svg)](https://golang.org)
  [![React version](https://img.shields.io/badge/react-..-dafb.svg)](https://react.dev)
</div>

---

  Overview

OpenRisk is a modern, enterprise-grade Risk Management Platform that transforms how organizations identify, assess, mitigate, and monitor risks. Built with a scalable microservices architecture, OpenRisk enables teams to move beyond spreadsheets and legacy systems into a seamless, automated risk management experience.

  What OpenRisk Enables

OpenRisk allows every organization to:
-  Identify IT & security risks
-  Score & Prioritize risks based on impact and probability
-  Track mitigation plans and action items
-  Monitor trends in real-time with interactive dashboards

  Designed For

- CTO & CISO - Strategic risk oversight and compliance
- DevSecOps - Integrated security in CI/CD pipelines
- Security Analysts - Risk assessment and investigation
- Compliance Teams - Audit trails and governance

  Key Advantages

-  Automated Risk Assessment - Reduce manual evaluation time
-  Interactive Dashboards - Real-time risk visualization
-  Native Integrations - Elastic, Splunk, TheHive, OpenCTI, AWS
-  Easy Deployment - Docker & Kubernetes ready
-  Enterprise Security - RBAC, SSO, audit logging
-  Scalable Architecture - Microservices-ready

 Key Capabilities
-  Risk Assessment - Comprehensive risk identification and scoring
-  Mitigation Tracking - Monitor and track risk mitigations in real-time
-  Advanced Analytics - Real-time dashboards and trend analysis
-  Enterprise Security - RBAC, audit logging, OAuth/SAML SSO
-  Integration Ready - TheHive, OpenCTI, Splunk, Elastic connectors
-  Custom Fields - Flexible schema for organizational needs
-  Gamification - Engagement and incentive system

---

  Quick Start ( Minutes)

 Prerequisites
- Docker & Docker Compose
- Git
- GB RAM, GB disk space

 Local Development

bash
 Clone the repository
git clone https://github.com/opendefender/OpenRisk.git
cd OpenRisk

 Start all services (PostgreSQL, Redis, Backend, Frontend)
docker compose up -d

 Access the application
 Frontend: http://localhost:
 Backend API: http://localhost:
 API Docs: http://localhost:/swagger


 Default Credentials

Email: admin@openrisk.local
Password: admin


---

  Technical Stack

 Backend
| Component | Technology | Version |
|-----------|-----------|---------|
| Language | Go | .. |
| Framework | Fiber | v. |
| Database | PostgreSQL |  |
| ORM | GORM | v. |
| Testing | Testify | v. |
| Architecture | CLEAN | Domain-Driven |

 Frontend
| Component | Technology | Version |
|-----------|-----------|---------|
| Framework | React | .. |
| State | Zustand | .. |
| Styling | Tailwind CSS | .. |
| Forms | React Hook Form | . |
| Routing | React Router | .. |
| Charts | Recharts | .. |

 Infrastructure
| Component | Technology | Purpose |
|-----------|-----------|---------|
| Containerization | Docker | Application packaging |
| Orchestration | Kubernetes | Production deployment |
| Charts | Helm | Ks configuration |
| CI/CD | GitHub Actions | Automated testing & deployment |
| Caching | Redis | Session & cache layer |

---

  Features

 Phase : Core Risk Management 
-  Risk CRUD operations (Create, Read, Update, Delete, List)
-  Risk scoring engine with weighted calculations
-  Mitigation tracking with checklist sub-actions
-  Asset management and relationships
-  Soft-delete support with audit trails

 Phase : Security & Authentication 
-  JWT-based authentication
-  API Token management (create, revoke, rotate)
-  Role-Based Access Control (RBAC) - Backend (+ endpoints,  domain models)
-  Permission matrices (resource-level granularity)
-  Comprehensive audit logging
-  OAuth/SAML SSO (Google, GitHub, Azure AD)

 Phase : Infrastructure & Deployment 
-  Docker Compose local development
-  GitHub Actions CI/CD pipeline
-  Integration test suite
-  Kubernetes Helm charts
-  Staging & production runbooks

 Phase .: RBAC Frontend Implementation 
-  Permission gate components ( reusable wrappers)
-  Route-level permission guards ( types)
-  Role & Tenant management pages (admin interfaces)
-  Advanced RBAC utilities (+ functions)
-  Audit logging system (compliance tracking)
-  Permission caching (performance optimization)
-  Custom React hooks (usePermissions, useAuditLog)
-  Comprehensive documentation (,+ lines)

 Phase : Enterprise Features 
-  Custom fields framework ( types)
-  Bulk operations with validation
-  Risk timeline (audit trail)
-  Advanced reporting & export

 Phase : Advanced Analytics 
-  Analytics dashboard with real-time data
-  Risk heatmaps and trend analysis
-  Incident management system
-  Threat tracking and mapping
-  Gamification & engagement system

 Phase : RBAC Frontend Enhancement 
-  Permission checking utilities (wildcard support, pattern matching)
-  Audit trail for compliance (event logging, filtering, export)
-  Performance optimization (permission caching with TTL)
-  Feature flag system (role-based feature enablement)
-  Comprehensive component library (+ components)

---

  Documentation

| Document | Purpose |
|----------|---------|
| [LOCAL_DEVELOPMENT.md](docs/LOCAL_DEVELOPMENT.md) | Setup guide for development environment |
| [API_REFERENCE.md](docs/API_REFERENCE.md) | Complete API endpoint documentation |
| [KUBERNETES_DEPLOYMENT.md](docs/KUBERNETES_DEPLOYMENT.md) | Ks deployment instructions |
| [PRODUCTION_RUNBOOK.md](docs/PRODUCTION_RUNBOOK.md) | Production operations guide |
| [INTEGRATION_TESTS.md](docs/INTEGRATION_TESTS.md) | Testing procedures |
| [SAML_OAUTH_INTEGRATION.md](docs/SAML_OAUTH_INTEGRATION.md) | SSO integration guide |
| [SYNC_ENGINE.md](docs/SYNC_ENGINE.md) | Integration sync documentation |
| [RBAC_FRONTEND_COMPONENTS_GUIDE.md](docs/RBAC_FRONTEND_COMPONENTS_GUIDE.md) | Frontend RBAC components & hooks |
| [RBAC_PHASE_COMPREHENSIVE_SUMMARY.md](docs/RBAC_PHASE_COMPREHENSIVE_SUMMARY.md) | Phase  implementation details |
| [ADVANCED_PERMISSIONS.md](docs/ADVANCED_PERMISSIONS.md) | RBAC & permissions documentation |

For more documentation, see the [docs](docs/) directory.

---

  Deployment

 Local Development
bash
docker compose up -d


 Staging Environment
bash
 See docs/STAGING_DEPLOYMENT.md
./scripts/deploy-kubernetes.sh --environment staging


 Production Deployment
bash
 See docs/PRODUCTION_RUNBOOK.md
helm install openrisk ./helm/openrisk \
  -f helm/values-prod.yaml \
  --namespace openrisk


---

  Testing

bash
 Run all tests
make test-all

 Backend unit tests
cd backend && go test ./...

 Frontend tests
cd frontend && npm test

 Integration tests
./scripts/run-integration-tests.sh


Test Statistics: + tests passing 

---

  API Overview

OpenRisk provides a comprehensive REST API with + endpoints:

 Core Endpoints

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


 RBAC & Security

POST   /auth/login             - JWT authentication
POST   /auth/register          - User registration
POST   /auth/oauth/:provider  - OAuth login
POST   /auth/saml/acs          - SAML assertion endpoint

GET    /api/tokens             - List API tokens
POST   /api/tokens             - Create new token
DELETE /api/tokens/:id         - Revoke token

GET    /rbac/roles             - List roles
POST   /rbac/roles             - Create role
PUT    /rbac/roles/:id         - Update role
DELETE /rbac/roles/:id         - Delete role
GET    /rbac/permissions       - List permissions

GET    /rbac/tenants           - List tenants
POST   /rbac/tenants           - Create tenant
GET    /rbac/tenants/:id/stats - Tenant statistics
DELETE /rbac/tenants/:id       - Delete tenant


 Analytics & Reporting

GET    /api/analytics/dashboard     - Dashboard metrics
GET    /api/analytics/trends        - Risk trends
GET    /api/reports                 - List reports
POST   /api/reports/export          - Export risks/mitigations


See [API_REFERENCE.md](docs/API_REFERENCE.md) for complete endpoint documentation with examples.

---

  Security

OpenRisk implements enterprise-grade security:

- Authentication: JWT tokens with expiration
- Authorization: RBAC with permission matrices
- Encryption: SHA hashing for sensitive data
- Audit: Complete audit trail for all operations
- SSO: OAuth and SAML support
- Rate Limiting: API rate limiting middleware
- Input Validation: Request validation with Zod/validator

See [ADVANCED_PERMISSIONS.md](docs/ADVANCED_PERMISSIONS.md) for detailed security documentation.

---

  Keyboard Shortcuts

OpenRisk includes keyboard shortcuts to help you work faster. Below is a complete list of available shortcuts:

 Global Shortcuts
| Shortcut | Action | Context |
|----------|--------|---------|
| <kbd>K</kbd> or <kbd>Ctrl+K</kbd> | Open global search | Anywhere in the app |
| <kbd>N</kbd> or <kbd>Ctrl+N</kbd> | Create new risk | Dashboard and Risks page |
| <kbd>Esc</kbd> | Close modal/dialog | Any open modal or dialog |

 Search & Navigation
| Shortcut | Action | Context |
|----------|--------|---------|
| <kbd>↑</kbd> | Previous search result | In search suggestions |
| <kbd>↓</kbd> | Next search result | In search suggestions |
| <kbd>Enter</kbd> | Select search result | Search suggestions open |
| <kbd>Esc</kbd> | Close search dropdown | Search suggestions open |

 Risk Management
| Shortcut | Action | Context |
|----------|--------|---------|
| <kbd>Esc</kbd> | Close risk details | Risk details panel open |
| <kbd>Esc</kbd> | Close edit modal | Risk editing modal open |

 Tips for Power Users

- Search Tip: Use <kbd>K</kbd> / <kbd>Ctrl+K</kbd> from anywhere to quickly search for risks, assets, or mitigations
- Quick Create: Press <kbd>N</kbd> / <kbd>Ctrl+N</kbd> on the dashboard to rapidly create new risks
- Navigation: Use arrow keys in search results to navigate without your mouse
- Mobile: These shortcuts work best on desktop/laptop keyboards

 Planned Shortcuts (Coming Soon)
- Edit Last Risk - <kbd>E</kbd> / <kbd>Ctrl+E</kbd>
- Filter Results - <kbd>F</kbd> / <kbd>Ctrl+F</kbd>
- Delete Selected - <kbd>D</kbd> / <kbd>Ctrl+D</kbd>
- Focus Search - <kbd>/</kbd> key
- Settings - <kbd>,</kbd> / <kbd>Ctrl+,</kbd>

---

  Contributing

We welcome contributions from the community! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

 Development Workflow
. Fork the repository
. Create a feature branch (git checkout -b feature/AmazingFeature)
. Commit your changes (git commit -m 'Add AmazingFeature')
. Push to the branch (git push origin feature/AmazingFeature)
. Open a Pull Request

---

  License

OpenRisk is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

---

  Support & Contact

- GitHub Issues: [Report bugs or request features](https://github.com/opendefender/OpenRisk/issues)
- Discussions: [Join community discussions](https://github.com/opendefender/OpenRisk/discussions)
- Security: See [SECURITY.md](SECURITY.md) for security vulnerability reporting

---

  Roadmap

 Q  - RBAC Frontend  (In Progress)
-  Permission gate components & hooks
-  Route-level permission guards
-  Role & tenant management pages
-  Audit logging system
-  Permission caching optimization
-  Code review & testing phase

 Q 
- [ ] Multi-tenant advanced features
- [ ] Permission analytics dashboard
- [ ] Role templates & bulk operations
- [ ] Mobile application (React Native)

 Q 
- [ ] Advanced RBAC enhancements
- [ ] Additional connector integrations
- [ ] Machine learning risk predictions
- [ ] API webhook support

 Q 
- [ ] Enterprise audit compliance
- [ ] Advanced analytics engine
- [ ] Custom dashboard builder
- [ ] Workflow automation

---

  Credits

OpenRisk is developed and maintained by the [OpenDefender](https://github.com/opendefender) community.

---

  Questions?

-  Check the [documentation](docs/)
-  Search existing [issues](https://github.com/opendefender/OpenRisk/issues)
-  Ask in [discussions](https://github.com/opendefender/OpenRisk/discussions)

---

<div align="center">
  Made with  by OpenDefender Community
  
  [ Star us on GitHub](https://github.com/opendefender/OpenRisk)
</div>
