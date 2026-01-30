 OpenRisk API Reference

Version: .. | Base URL: /api/v | Auth: Bearer JWT

 Endpoints

 Health
- GET /health - Health check

 Authentication
- POST /auth/login - Login (email, password)
- GET /users/me - Current user profile

 Risks
- GET /risks - List risks (query: page, limit, sort_by)
- POST /risks - Create risk
- GET /risks/{id} - Get risk
- PATCH /risks/{id} - Update risk
- DELETE /risks/{id} - Delete risk

 Mitigations
- POST /risks/{id}/mitigations - Add mitigation
- PATCH /mitigations/{mitigationId} - Update mitigation
- PATCH /mitigations/{mitigationId}/toggle - Toggle status (PLANNED↔DONE)

 Mitigation Sub-Actions
- POST /mitigations/{id}/subactions - Create sub-action
- PATCH /mitigations/{id}/subactions/{subactionId}/toggle - Toggle completion
- DELETE /mitigations/{id}/subactions/{subactionId} - Delete sub-action
- GET /mitigations/recommended - Get recommended mitigations

 Assets
- GET /assets - List assets
- POST /assets - Create asset

 Statistics
- GET /stats - Dashboard stats
- GET /stats/risk-matrix - Risk matrix (impact vs probability)
- GET /stats/trends - Risk trends

 Export
- GET /export/pdf - Export risks to PDF

 Gamification
- GET /gamification/me - User gamification profile

---

 Request/Response Examples

See docs/openapi.yaml for complete OpenAPI . specification with detailed schemas, validation rules, and examples.

 Authentication

All protected endpoints require:

Authorization: Bearer {token}


Obtain token via POST /auth/login (valid for  hours).

 Error Format

json
{
  "error": "Error message",
  "code": ,
  "details": {}
}


Common codes:  (Bad Request),  (Unauthorized),  (Not Found),  (Server Error)

---

Full specification: [openapi.yaml](./openapi.yaml)  
Last updated: December , 
