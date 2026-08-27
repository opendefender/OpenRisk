---
name: openrisk-backend-charter
description: OpenRisk backend engineering charter — Clean Architecture layering, tenant isolation patterns, typed errors, transactions, migrations, scoring integration and the backend definition of done. Load before implementing or reviewing Go code.
---

# OpenRisk Backend Charter

## Layering — dependency direction is inward only

```
/cmd/server/main.go        DI container, graceful shutdown
/internal/domain/          pure entities. A gorm.io or fiber import here is a merge blocker.
/internal/application/     one use case per file. Declares repository interfaces.
/internal/infrastructure/  GORM repositories, Redis, messaging, integrations
/internal/api/http/        Fiber handlers, DTOs, middleware, RBAC declarations
/pkg/                      scoring, cti, notify, export, ai, crq — pure, testable
```

## Tenant isolation — the pattern

Every repository method takes the tenant from the context and puts it in the
WHERE clause:
```go
func (r *GormRiskRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.Risk, error) {
    tenantID, ok := middleware.TenantFromContext(ctx)
    if !ok {
        return nil, domain.ErrForbidden // fail-closed, never uuid.Nil
    }
    var risk domain.Risk
    if err := r.db.WithContext(ctx).
        Where("id = ? AND tenant_id = ?", id, tenantID).
        First(&risk).Error; err != nil { ... }
}
```

Table without a `tenant_id` column (e.g. `risk_histories`): gate through the
parent with an explicit ownership check, and JOIN for list queries.
```go
func (s *Service) ownsRisk(ctx context.Context, riskID uuid.UUID) error
```

Sequential integer IDs are enumerable. Any handler taking one calls `ownsX`
before reading or writing. Return `404` on a foreign object, never `403` —
do not confirm the object exists.

## Typed errors — the only four

`ErrNotFound` · `ErrForbidden` · `ErrConflict` · `ErrValidation`.
Wrap with context: `fmt.Errorf("application.CreateRisk: %w", err)`.
Mapped to HTTP status in exactly one place. Never inline in a handler.

## Tests — minimum per use case

`TestXxx_Success` · `TestXxx_NotFound` · `TestXxx_Unauthorized`
plus, for anything with a tenant dimension, `TestXxx_CrossTenant` proving
tenant A cannot reach tenant B's object by ID.

Table-driven. Test names describe behaviour:
`TestCreateRisk_RejectsDuplicateReference`, not `TestCreateRisk2`.

## Migrations

golang-migrate, forward-only, one concern per file, numbered sequentially.
Adding a `tenant_id` to an existing table requires a backfill step and a new
unique index scoped by tenant. Test against a fresh database before reporting.
Any new model: confirm it is registered in `AutoMigrate` or explain why not.

## Definition of Done — backend

1. `gofmt -l .` returns nothing
2. `go vet ./...` clean
3. `go test ./... -race -count=1` green, no new failures
4. Every new query re-read for its tenant filter
5. Cross-tenant test written for every new endpoint
6. Typed errors, no raw `errors.New` escaping the domain
7. OpenAPI spec updated if a route changed
