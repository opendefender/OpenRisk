 Changelog

All notable changes to OpenRisk will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/../).

 [Unreleased]

 Planned
- Multi-tenant support
- Mobile app (React Native)
- Slack/Teams notifications
- Jira integration

 [..] - --

 Added
- Analytics dashboard with real-time risk metrics
- Gamification system with badges and progress tracking
- Custom fields framework ( field types supported)
- Bulk operations for risks and mitigations
- Advanced search and filtering capabilities
- Risk timeline view (audit trail)

 Improved
- Dashboard load time reduced by %
- Mobile responsive design across all pages
- API response times optimized
- Documentation structure reorganized

 Fixed
- API token expiration edge cases
- Search filter bugs with special characters
- Session handling on token refresh
- Mobile menu navigation issues

 [..] - --

 Added
- OAuth/SAML SSO support (Google, GitHub, Azure AD)
- Role-Based Access Control (RBAC)
- API token management (create, revoke, rotate)
- Comprehensive audit logging

 Improved
- Authentication flow security
- Permission matrix granularity
- Database query optimization

 Fixed
- JWT token refresh bugs
- Permission check edge cases

 [..] - --

 Added
- Mitigation sub-actions (checklist items)
- Asset relationship management
- Risk scoring engine improvements

 Fixed
- Soft-delete cascade issues
- Asset linking bugs

 [..] - --

 Added
- Basic CRUD for risks, mitigations, assets
- Initial dashboard
- Documentation structure

 [..] - --

 Added
- Initial release
- Core risk management features
- React frontend + Go backend
- Docker Compose setup
- Basic authentication

---

[Unreleased]: https://github.com/opendefender/OpenRisk/compare/.....HEAD
[..]: https://github.com/opendefender/OpenRisk/compare/.......
[..]: https://github.com/opendefender/OpenRisk/compare/.......
