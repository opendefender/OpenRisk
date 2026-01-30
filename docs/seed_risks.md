 Seeding Risks (development)

This document explains how to seed the risks table locally using the fixtures in dev/fixtures/risks.json.

Prerequisites
- running Postgres instance
- psql CLI available and network access to the DB

Example (psql):

. Load fixtures using jq + SQL insert (simple, idempotent for dev only):

bash
DB_CONN="postgresql://user:password@localhost:/openrisk_dev"
cat dev/fixtures/risks.json | jq -c '.[]' | while read row; do
  id=$(echo "$row" | jq -r '.id')
  title=$(echo "$row" | jq -r '.title' | sed "s/'/''/g")
  desc=$(echo "$row" | jq -r '.description' | sed "s/'/''/g")
  impact=$(echo "$row" | jq -r '.impact')
  prob=$(echo "$row" | jq -r '.probability')
  score=$(echo "$row" | jq -r '.score')
  status=$(echo "$row" | jq -r '.status')
  tags=$(echo "$row" | jq -r '.tags | @text')
  created=$(echo "$row" | jq -r '.created_at')
  updated=$(echo "$row" | jq -r '.updated_at')

  psql "$DB_CONN" -c "INSERT INTO risks (id, title, description, impact, probability, score, status, tags, created_at, updated_at) \
    VALUES ('${id}', '${title}', '${desc}', ${impact}, ${prob}, ${score}, '${status}', '{${tags}}', '${created}', '${updated}') \
    ON CONFLICT (id) DO UPDATE SET title = EXCLUDED.title;"

done


Notes
- This is a convenience script for local development only. For production, use proper migration/seed tooling (e.g. goose, migrate, or a Go seed command which validates inputs).
- Example uses jq to unwrap JSON. Adjust quoting for your shell.
