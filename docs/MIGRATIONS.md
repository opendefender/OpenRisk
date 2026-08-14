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

## Dette connue (à résorber — suivi P0.5 #3/#4 du plan de lancement)

Le rail SQL « marche par accident » : golang-migrate lit le dossier **racine** `migrations/`, y
**ignore** les fichiers legacy mal formés (`0001`–`0024`, des `.sql` nus sans `.up/.down`, dont un
**doublon de version `0018`**), et applique les bien formés (`0048`–`0054`) — se marquant `version=54`.
Deux pièges documentés :

- **`backend/migrations/` (0026–0047) n'est JAMAIS lu** par le runner (dossier orphelin). Son contenu
  (ADD COLUMN redondant avec AutoMigrate + backfills legacy) n'a d'effet que sur une base pré-existante.
- **Aucun trigger append-only** n'existe sur `audit_events` / `admin_audit_events` (0 trigger vérifié
  sur base fraîche). L'immuabilité de la piste d'audit ne repose aujourd'hui que sur le **hash-chain
  applicatif** (`audit_chain.go`), pas sur une garantie base. La **RÈGLE #4** du Master Prompt V5
  (« trigger PG rejetant UPDATE/DELETE ») n'est donc pas tenue au niveau DB. ⚠️ Un tel trigger doit
  **autoriser la purge de rétention** (le worker de rétention supprime des events) — d'où sa
  conception délicate (rôle/flag de session dédié, pas un blocage aveugle des DELETE).

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
