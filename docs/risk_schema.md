 Risk Schema Design

This document defines the Risk domain schema, DB migration plan, backend domain model outline, TypeScript interfaces, and sample fixtures.

 Goals
- Store structured risks with fields required for calculation, filtering and integrations.
- Keep schema flexible (arrays, JSONB for custom fields) while enabling efficient queries (indexes for score, status, tags).
- Support relations: assets (many-to-many), mitigations (one-to-many), history/events.

 Risk core fields
- id: UUID PRIMARY KEY
- title: TEXT NOT NULL
- description: TEXT
- impact: INTEGER NOT NULL (-)
- probability: INTEGER NOT NULL (-)
- score: NUMERIC(,) NOT NULL — computed field (probability × impact × asset_criticality factor)
- status: VARCHAR NOT NULL (OPEN, MITIGATED, ACCEPTED, MONITORING, CLOSED)
- tags: TEXT[]
- frameworks: TEXT[] (ISO, NIST, CIS, OWASP...)
- source: TEXT (origin of the risk: UI, OpenCTI, TheHive, import)
- custom_fields: JSONB NULL — flexible store for tenant-specific fields
- created_at: TIMESTAMP WITH TIME ZONE DEFAULT now()
- updated_at: TIMESTAMP WITH TIME ZONE DEFAULT now()

 Relations
- Risk ↔ Asset: many-to-many via risk_assets(risk_id UUID, asset_id UUID)
- Risk ↔ Mitigation: one-to-many, mitigations.risk_id → risks.id
- Risk history stored in risk_history table with events for auditing

 Indexes
- index on score (for sorting)
- index on status
- GIN index on tags and frameworks (text[])
- GIN index on custom_fields (jsonb) if used heavily

 DB Migration Plan (high level)
) Create risks table (see migration SQL in migrations/_create_risks_table.sql)
) Create risk_assets join table ()
) Create mitigations table if not present
) Add indexes and constraints
) Add triggers or application-level hooks to compute score on insert/update

 Backend domain models (outline)
- Risk struct (Go):
  - ID uuid.UUID gorm:"type:uuid;primaryKey"
  - Title string
  - Description string
  - Impact int
  - Probability int
  - Score float
  - Status string
  - Tags pq.StringArray gorm:"type:text[]"
  - Frameworks pq.StringArray gorm:"type:text[]"
  - Source string
  - CustomFields datatypes.JSON
  - CreatedAt time.Time
  - UpdatedAt time.Time
  - Assets []Asset gorm:"manymany:risk_assets"
  - Mitigations []Mitigation

Scoring: keep calculation in a small service score.Calculate(risk, assets) so it can be unit-tested and reused by handlers.

 TypeScript interfaces (frontend)
See frontend/src/types/risk.ts for the canonical TS interface.

 Fixtures & seeds
- dev/fixtures/risks.json contains example risks used for development and UI testing.
- Provide a SQL seed or small script to load fixtures into a local dev database.

 Notes
- For tenant isolation, add tenant_id to each table (if multi-tenant planned).
- For integration imports, store source_id (external ID) and source (system) to enable deduplication.

---

Files added alongside this design:
- migrations/_create_risks_table.sql
- migrations/_create_risk_assets_table.sql
- frontend/src/types/risk.ts
- dev/fixtures/risks.json
- docs/seed_risks.md
