# Migrations & schema authority

> Établi le 2026-08-14 par vérification **empirique** (boot du serveur contre une base Postgres 15
> fraîche isolée). Sources de vérité du code : `backend/cmd/server/main.go` (bloc AutoMigrate,
> ~l.178–334) et `backend/internal/migrations/migrator.go`.

## TL;DR

1. **`AutoMigrate` (GORM) est l'autorité de schéma.** Au boot, il crée/aligne l'intégralité des
   tables depuis les ~80 modèles Go listés dans `main.go`. Preuve : sur une base **vierge**, le boot
   produit **74 tables** et le serveur écoute — sans aucune migration SQL préalable.
2. **La couche SQL (`migrations/`) est *additive*, appliquée APRÈS AutoMigrate.** Elle ne porte que
   ce qu'AutoMigrate ne sait pas exprimer : triggers, contraintes CHECK, et backfills de données
   ponctuels. Elle est exécutée par `migrations.RunMigrations()` via golang-migrate.
3. **`RunMigrations` échoue désormais bruyamment** (depuis 2026-08-14) : une erreur d'initialisation
   ou d'application **avorte le boot** (`log.Fatalf`). Avant, l'erreur était avalée (`log.Printf`) →
   le serveur pouvait démarrer sur un schéma à moitié migré, silencieusement. Seuls « pas de
   `DATABASE_URL` » et `ErrNoChange` (déjà à jour) ne sont pas des erreurs.

## Conséquence pour un déploiement neuf (le cas du lancement)

Une base prod **fraîche** n'a **aucune donnée legacy** → les backfills de données sont des no-op, et
AutoMigrate produit le schéma complet et correct. **Le premier déploiement propre ne dépend pas des
migrations SQL pour son schéma.** (C'est pourquoi le « la base dev est dirty à v40 » n'a jamais
empêché le produit de tourner : AutoMigrate faisait tout le travail.)

## Configuration

- `DATABASE_URL` — `postgres://user:pass@host:port/db?sslmode=disable`. Absente → couche SQL sautée.
- `MIGRATIONS_DIR` — dossier lu par golang-migrate (défaut `migrations`, relatif au CWD du process).

## État de la couche SQL (P0.5, 2026-08-14)

Le dossier lu ne contient plus que les migrations **bien formées et exécutées** (`0048`+). Le reste a
été déplacé sous `migrations/_archive/` (ignoré par le runner, non-récursif) :
- **`_archive/root-legacy-preautomigrate/`** — les `0001`–`0024` en `.sql` nus (mauvais format, déjà
  ignorés ; redondants avec AutoMigrate ; portaient un **doublon de version `0018`**, désamorcé).
- **`_archive/backend-0026-0047-neverread/`** — l'ancien `backend/migrations/`, jamais lu par le runner
  (ADD COLUMN redondant + backfills legacy sans objet sur base fraîche).

### Append-only DB sur la piste d'audit — RÈGLE #4 (migration `0055`)
Un trigger `BEFORE UPDATE OR DELETE` (`openrisk_audit_append_only`) sur `audit_events` **et**
`admin_audit_events` **rejette toute mutation** d'une entrée déjà écrite. Escape unique pour la
maintenance système : une transaction qui pose `SET LOCAL openrisk.audit_maintenance = 'on'` peut
muter — utilisé par exactement deux chemins auditables (`GormAuditChainRepository.Prune` = purge de
rétention ; `PrepareForAutoMigrate` = backfill legacy). Toute écriture applicative ordinaire ne pose
jamais ce GUC → UPDATE/DELETE refusés. Prouvé live sur Postgres : INSERT ok, UPDATE/DELETE rejetés,
DELETE sous GUC autorisé ; `up`/`down` réversibles. (Sur sqlite — utilisé par les tests — le GUC est
sauté par garde de dialecte et aucun trigger n'existe.)

## Recettes

**Débloquer une base dev *dirty*** (golang-migrate refuse de tourner sur un état sale) :
```bash
# inspecter
psql "$DATABASE_URL" -c "select * from schema_migrations;"
# forcer la version propre connue-bonne, puis relancer le boot
migrate -path migrations -database "$DATABASE_URL" force 54
```

**Ajouter une migration** (couche additive, format golang-migrate obligatoire) :
```
migrations/00NN_courte_description.up.sql
migrations/00NN_courte_description.down.sql
```
Régles : idempotent (`... IF NOT EXISTS`), s'appuie sur les tables qu'AutoMigrate a déjà créées,
`down` réversible, testé `up` **et** `down` sur une base fraîche avant merge.
