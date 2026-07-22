# OpenRisk — RBAC, rôles métiers & tableaux de bord

_Branche `feature/rbac-business-roles` — 2026-07-22._

Ce document décrit le modèle d'autorisation d'OpenRisk après l'ajout des **rôles
métiers GRC**, la matrice de permissions, les vues par profil, et les décisions
d'architecture. Il fait aussi office de **rapport récapitulatif** (audit +
travaux réalisés + limites) demandé.

---

## 1. Modèle d'autorisation (runtime)

Le chemin d'autorisation **effectif** — celui qui décide de chaque requête — est :

```
OrganizationMember.Role (root | admin | user)
        │
        │  login (internal/application/auth/login.go)  →  OrganizationMember.EffectivePermissions()
        ▼
JWT { org_roles: {orgId: role}, permissions: []string }   (RS256, pkg/auth)
        │
        ▼
middleware.RequirePermission("risks:read", …)   /   middleware.RequireRole("admin","root")
        │   (wildcard-aware : "*" et "resource:*")
        ▼
handler (repository filtrant TOUJOURS par tenant_id — RÈGLE #2)
```

- **`root` / `admin`** → `permissions = ["*"]` (accès complet). `root` = propriétaire
  de l'organisation.
- **`user`** → permissions résolues depuis son **rôle métier** (préset) et/ou un
  éventuel `Profile` hérité, unifiés et dédupliqués par `EffectivePermissions()`.

### Trois rôles d'organisation, N rôles métiers

Le rôle d'organisation (`root`/`admin`/`user`) reste volontairement minimal. La
granularité « métier » est portée par le champ **`OrganizationMember.BusinessRole`**
(un préset parmi ceux ci-dessous). Un `user` sans rôle métier n'a aucune
permission ; un `user` avec un rôle métier reçoit exactement les permissions du
préset.

---

## 2. Les profils demandés → rôles OpenRisk

| Profil demandé | Rôle OpenRisk | Type |
|---|---|---|
| **Tenant Admin** (administrateur client) | rôle d'organisation `admin` (`["*"]`) | org role |
| RSSI | `rssi` | rôle métier |
| DSI | `dsi` | rôle métier |
| Risk Manager | `risk_manager` | rôle métier |
| Auditeur | `auditor` | rôle métier |
| Responsable conformité | `compliance_officer` | rôle métier |
| Contrôle interne | `internal_control` | rôle métier |
| Asset Owner | `asset_owner` | rôle métier |
| Risk Owner | `risk_owner` | rôle métier |
| Analyste sécurité | `security_analyst` | rôle métier |
| **Direction** (Executive Dashboard) | `executive` | rôle métier |
| Lecteur (read-only) | `viewer` | rôle métier |

> **Tenant Admin.** Le rôle « administrateur client » correspond au rôle
> d'organisation `admin`, qui a le wildcard `*` : administration de
> l'organisation, gestion des utilisateurs/équipes/rôles, politiques GRC,
> intégrations, notifications, paramètres, licences/abonnements, journaux
> d'audit, API Keys (PAT) & Webhooks. L'isolation multi-tenant est garantie par
> le filtrage `tenant_id` au niveau des repositories (RÈGLE #2) — un admin ne
> voit jamais les données d'un autre tenant.

---

## 3. Matrice de permissions (source de vérité générée depuis l'API)

`✓` = permission accordée · `·` = non accordée. Les permissions destructives
laissées vides pour **tous** les rôles métiers (`compliance:*:delete`,
`reports:board:approve/delete`) sont **réservées au Tenant Admin** (wildcard).

<!-- généré depuis GET /rbac/business-roles -->

| Permission | rssi | dsi | risk_manager | auditor | compliance_officer | internal_control | asset_owner | risk_owner | security_analyst | executive | viewer |
|---|---|---|---|---|---|---|---|---|---|---|---|
| **risks** | | | | | | | | | | | |
| `risks:read` | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |
| `risks:create` | ✓ | ✓ | ✓ | · | · | · | · | · | ✓ | · | · |
| `risks:update` | ✓ | ✓ | ✓ | · | · | · | · | ✓ | ✓ | · | · |
| `risks:delete` | · | · | ✓ | · | · | · | · | · | · | · | · |
| **assets** | | | | | | | | | | | |
| `assets:read` | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | · | ✓ |
| `assets:create` | · | ✓ | · | · | · | · | ✓ | · | · | · | · |
| `assets:update` | · | ✓ | · | · | · | · | ✓ | · | · | · | · |
| `assets:delete` | · | ✓ | · | · | · | · | · | · | · | · | · |
| **mitigations** | | | | | | | | | | | |
| `mitigations:read` | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | · | ✓ |
| `mitigations:create` | ✓ | ✓ | ✓ | · | · | · | · | ✓ | ✓ | · | · |
| `mitigations:update` | ✓ | ✓ | ✓ | · | · | ✓ | ✓ | ✓ | ✓ | · | · |
| `mitigations:delete` | · | · | ✓ | · | · | · | · | · | · | · | · |
| **vulnerabilities** | | | | | | | | | | | |
| `vulnerabilities:read` | ✓ | ✓ | ✓ | ✓ | · | · | ✓ | ✓ | ✓ | · | ✓ |
| `vulnerabilities:update` | ✓ | · | · | · | · | · | · | · | ✓ | · | · |
| `vulnerabilities:delete` | · | · | · | · | · | · | · | · | ✓ | · | · |
| **incidents** | | | | | | | | | | | |
| `incidents:read` | ✓ | ✓ | · | ✓ | · | · | · | · | ✓ | · | ✓ |
| `incidents:create` | ✓ | ✓ | · | · | · | · | · | · | ✓ | · | · |
| `incidents:update` | ✓ | ✓ | · | · | · | · | · | · | ✓ | · | · |
| `incidents:delete` | · | · | · | · | · | · | · | · | ✓ | · | · |
| **compliance** | | | | | | | | | | | |
| `compliance:read` | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |
| `compliance:frameworks:read` | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | · | · | · | · | ✓ |
| `compliance:frameworks:create` | · | · | · | · | ✓ | · | · | · | · | · | · |
| `compliance:frameworks:delete` | · | · | · | · | · | · | · | · | · | · | · |
| `compliance:controls:read` | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | · | · | ✓ | · | ✓ |
| `compliance:controls:create` | · | · | · | · | ✓ | · | · | · | · | · | · |
| `compliance:controls:update` | ✓ | · | · | · | ✓ | ✓ | · | · | · | · | · |
| `compliance:controls:delete` | · | · | · | · | · | · | · | · | · | · | · |
| `compliance:evidences:read` | ✓ | · | · | ✓ | ✓ | ✓ | · | · | · | · | · |
| `compliance:evidences:create` | · | · | · | · | ✓ | ✓ | · | · | · | · | · |
| `compliance:evidences:delete` | · | · | · | · | · | · | · | · | · | · | · |
| `compliance:audits:read` | · | · | · | ✓ | ✓ | ✓ | · | · | · | · | · |
| `compliance:audits:write` | · | · | · | ✓ | ✓ | ✓ | · | · | · | · | · |
| `compliance:remediations:read` | · | · | · | ✓ | ✓ | ✓ | · | · | · | · | · |
| `compliance:remediations:write` | · | · | · | · | ✓ | ✓ | · | · | · | · | · |
| **scanner** | | | | | | | | | | | |
| `scanner:read` | ✓ | ✓ | · | · | · | · | · | · | ✓ | · | · |
| `scanner:create` | · | ✓ | · | · | · | · | · | · | ✓ | · | · |
| `scanner:scan` | ✓ | ✓ | · | · | · | · | · | · | ✓ | · | · |
| `scanner:import` | ✓ | ✓ | · | · | · | · | · | · | ✓ | · | · |
| `scanner:delete` | · | · | · | · | · | · | · | · | ✓ | · | · |
| **automation** | | | | | | | | | | | |
| `automation:read` | ✓ | ✓ | ✓ | · | · | · | · | · | ✓ | · | · |
| `automation:write` | ✓ | · | · | · | · | · | · | · | ✓ | · | · |
| **reports** | | | | | | | | | | | |
| `reports:board:read` | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | · | · | · | ✓ | ✓ |
| `reports:board:create` | · | · | ✓ | · | · | · | · | · | · | · | · |
| `reports:board:update` | · | · | ✓ | · | · | · | · | · | · | · | · |
| `reports:board:approve` | · | · | · | · | · | · | · | · | · | · | · |
| `reports:board:delete` | · | · | · | · | · | · | · | · | · | · | · |

Nombre de permissions par rôle : rssi 23 · dsi 23 · security_analyst 23 ·
risk_manager 17 · compliance_officer 16 · internal_control 15 · auditor 13 ·
viewer 9 · asset_owner 8 · risk_owner 8 · executive 3.

---

## 4. Navigation, dashboards et landing par profil

La navigation (barre latérale + palette ⌘K) est **filtrée par permission**
(`visibleNavGroups`) : un rôle ne voit que les écrans que l'API l'autorise à
atteindre. Après connexion, chaque rôle atterrit sur un écran pertinent
(`landingForBusinessRole`, miroir de `domain.DefaultLandingFor`).

| Rôle | Landing | Menus visibles (extrait) |
|---|---|---|
| RSSI | `/` (dashboard sécurité) | Dashboard, Analytics, Risques, Vulnérabilités, Incidents, Automatisation, Compliance, CTI, Actifs, Reports |
| DSI | `/assets` | Dashboard, Risques, Actifs (CRUD), Vulnérabilités, Incidents, Scanner, Compliance |
| Risk Manager | `/risks` | Dashboard, Risques (CRUD), Mitigations, Reports (board create/update) |
| Auditeur | `/compliance` | Compliance (audits + remédiations read), Preuves, Risques (read) |
| Responsable conformité | `/compliance` | Compliance (frameworks/controls/evidences/audits/remédiations) |
| Contrôle interne | `/compliance` | Compliance (test des contrôles, audits, remédiations) |
| Asset Owner | `/assets` | Actifs (CRUD limité), Risques/Vulns (read) |
| Risk Owner | `/risks` | Risques (update), Mitigations |
| Analyste sécurité | `/vulnerabilities` | Vulnérabilités, Incidents, Scanner, Automatisation |
| **Direction** | `/analytics` | **Tableau de bord exécutif** + Financier + Reports (read) |
| Lecteur | `/` | Tout en lecture |

### Tableau de bord Direction (Executive)

La vue **Direction** (`/analytics`, `ExecutiveDashboard.tsx`) est un tableau de
bord **stratégique et visuel** déjà en place (spec §11) : cyber score A–F,
exposition financière (ALE FCFA), top-10 risques, répartition par criticité,
évolution du risque, couverture de conformité, KRI (dont MTTR), tendance
incidents. Le rôle `executive` est volontairement en **lecture seule stratégique**
(`risks:read`, `compliance:read`, `reports:board:read`) et landé directement
dessus — pas de détail technique.

---

## 5. API RBAC (nouveaux endpoints)

| Méthode & route | Garde | Rôle |
|---|---|---|
| `GET /rbac/business-roles` | authentifié | catalogue permissions + présets (matrice) |
| `GET /rbac/members` | `RequireRole(admin)` | membres du tenant + accès résolu |
| `PUT /rbac/members/:userId/business-role` | `RequireRole(admin)` | (ré)assigner un rôle métier, option. changer le rôle org |

Body d'affectation : `{ "business_role": "rssi", "member_role": "user" }`.
`business_role: ""` efface le rôle. Le propriétaire (`root`) est protégé de toute
modification via cet endpoint.

Écran d'administration : **`/settings/roles`** (« Rôles & accès », admin
uniquement) — onglet **Rôles & permissions** (matrice) + onglet **Membres**
(affectation inline).

---

## 6. Décisions d'architecture

1. **Un seul vocabulaire de permissions.** Le catalogue
   (`internal/domain/business_roles.go`) est exprimé dans les **mêmes chaînes**
   que les gardes `RequirePermission(...)`. Impossible qu'un préset accorde une
   permission qu'aucune route ne vérifie — garanti par `ValidateBusinessRoles()`
   (test).
2. **Domaine pur.** Catalogue + présets sans dépendance GORM/Fiber → testables et
   partagés tels quels entre la résolution backend et l'API du frontend.
3. **`EffectivePermissions()` = point unique.** Login **et** rafraîchissement de
   token résolvent les permissions au même endroit ; un changement de rôle prend
   effet au prochain refresh.
4. **Réutilisation.** On étend le modèle `organization_members` existant (champ
   `business_role`) plutôt que de créer un système parallèle. Les deux systèmes
   RBAC hérités (`domain/rbac.go` / `RoleService` / `RequirePermissions` pluriel,
   `service.PermissionService`) restent en place pour les écrans d'administration
   mais **ne pilotent pas** l'autorisation runtime.
5. **Extensibilité.** Ajouter un rôle = ajouter une entrée dans `businessRoles`
   (backend) ; il apparaît automatiquement dans l'API, la matrice, le sélecteur
   d'affectation et le libellé de la barre latérale. Aucun changement de handler
   ni de route.
6. **Incidents alignés.** Les écritures d'incident, jadis gardées par
   `RequireRole("admin","analyst")` (où « analyst » n'est pas un rôle runtime,
   donc admin-only de fait), passent au vocabulaire de permissions `incidents:*`.
7. **Least privilege.** Les actions destructives cross-tenant (suppression de
   référentiel/contrôle/preuve, approbation/suppression de rapport Comex) restent
   réservées au Tenant Admin.

---

## 7. Rapport récapitulatif

### Déjà présent (audit)
- Chemin runtime JWT RS256 + `RequirePermission`/`RequireRole` (wildcard-aware),
  isolation `tenant_id` au niveau repo.
- Tenant Admin (rôle `admin` = `*`) : users/roles/tenants (`/rbac/*`), audit,
  gouvernance, PAT (`/auth/pat`), intégrations, canaux de notification, settings.
- Tableau de bord exécutif (`/analytics`) + financier (`/analytics/financial`).

### Ajouté
- **11 rôles métiers GRC** (least-privilege) + catalogue de 46 permissions,
  source de vérité pure (`internal/domain/business_roles.go`).
- Résolution des rôles métiers au **login et au refresh**
  (`OrganizationMember.EffectivePermissions()`), champ `business_role` renvoyé au
  front.
- **API** : `/rbac/business-roles`, `/rbac/members`,
  `PUT /rbac/members/:userId/business-role`.
- **Frontend** : permissions réelles décodées du JWT (`lib/jwt.ts`), navigation
  filtrée par permission, **landing par rôle**, écran **`/settings/roles`**
  (matrice + affectation), libellé de rôle dans la barre latérale.
- **Correctif** : incidents en permissions (`incidents:*`) ; **fuite de garde
  marketplace** (`protected.Use(RequireRole)`) qui bloquait tout rôle métier sur
  automation/CTI/scanner/gouvernance/RBAC — désormais scopée.

### Fichiers principaux
- Backend : `internal/domain/business_roles.go` (+ test),
  `internal/domain/membership.go` (`BusinessRole`, `EffectivePermissions`),
  `internal/application/auth/login.go`,
  `internal/application/rbac/business_role_usecases.go` (+ test),
  `internal/infrastructure/repository/gorm_member_rbac_repository.go`,
  `internal/handler/rbac_business_role_handler.go`, `cmd/server/main.go`,
  `migrations/0038_add_member_business_role.{up,down}.sql`.
- Frontend : `src/lib/jwt.ts`, `src/hooks/useAuthStore.ts`,
  `src/shared/navModel.ts`, `src/components/layout/{Sidebar,CommandPalette}.tsx`,
  `src/features/auth/AuthScreen.tsx`, `src/features/rbac/*`, `src/App.tsx`.

### Migrations
- `0038_add_member_business_role` : colonne `organization_members.business_role`
  (varchar 64, nullable, indexée). Auto-migrée par GORM également.

### Tests ajoutés
- `internal/domain/business_roles_test.go` (validité catalogue, least-privilege
  viewer/executive, copies défensives).
- `internal/domain/membership_test.go` (`EffectivePermissions` : wildcard, préset,
  bare-user vide, union dédupliquée profil+rôle).
- `internal/application/rbac/business_role_usecases_test.go` (affectation :
  succès, rôle inconnu 400, introuvable 404, isolation cross-tenant, root
  protégé, downgrade, rôle org invalide, list résout les permissions).

### Preuve live (binaire :8090, Postgres:5434 + Redis)
Utilisateur `user` + rôle métier **RSSI** :
- affectation via API → `business_role: rssi`, `org_role: user`, **23 permissions
  résolues** ; rôle inconnu → **400**.
- login → `business_role: rssi` dans la réponse, JWT `permissions` = 23 (préset),
  pas de wildcard.
- gardes de routes réelles : `GET /risks`, `/automation/rules`,
  `/cti/vulnerabilities`, `/vulnerabilities`, `/compliance/frameworks`,
  `/rbac/business-roles` → **200** ; `POST /incidents` → passe la garde ;
  `DELETE /risks/:id` → **403** (pas de `risks:delete`) ; `/rbac/members`,
  `/marketplace/apps`, `/governance/audit-events` → **403** ; admin inchangé.

### Limites / améliorations futures
- **Portée « own/assigned »** : les présets Asset Owner / Risk Owner accordent
  aujourd'hui un accès `read/update` à l'échelle du tenant, pas restreint aux
  seuls objets dont ils sont propriétaires (le scope `own`/`assigned` existe dans
  le modèle `Profile` mais n'est pas encore appliqué au niveau requête). À
  implémenter via un filtrage owner dans les repos.
- **Rôles métiers personnalisés par tenant** : les présets sont globaux
  (code) ; un tenant ne peut pas encore composer son propre rôle via l'UI (le
  modèle `organization_roles` existe et pourrait être branché).
- **Gardes de route côté frontend** : la navigation masque les écrans
  interdits et le backend applique l'autorisation ; un accès direct par URL
  charge la page mais l'API renvoie 403 (dégradation gracieuse). Un
  `PermissionRoute` par écran serait plus net.
- **Deux vocabulaires `RiskStatus`** et les deux systèmes RBAC hérités non
  unifiés (dette pré-existante, hors périmètre).
