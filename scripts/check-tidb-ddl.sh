#!/usr/bin/env bash
# Pre-merge gate: verifies that the current branch's GORM models produce DDL
# that TiDB accepts, by running AutoMigrate (via MIGRATE_ONLY=true) against a
# throwaway TiDB instance — the same Dockerfile.tidb used for the prod target.
#
# Typical flow:
#   - feature branch developed/tested against MySQL (docker-compose.yml)
#   - before merging to master (which runs on TiDB), run this script
#   - if it fails, the new/changed models need adjusting for TiDB
#     (e.g. AUTO_INCREMENT -> AUTO_RANDOM, unsupported ALTER combos, etc.)
#
# Usage:
#   ./scripts/check-tidb-ddl.sh        # run the check
#   ./scripts/check-tidb-ddl.sh down   # tear down the throwaway TiDB instance
set -euo pipefail
cd "$(dirname "$0")/.."

COMPOSE=(docker compose -f docker-compose.yml -f docker-compose.migration.yml)

if [[ "${1:-}" == "down" ]]; then
  "${COMPOSE[@]}" down -v tidb tidb-init tidb-schema-check
  exit 0
fi

echo "[check-tidb-ddl] Building backend image from current branch..."
"${COMPOSE[@]}" build backend

echo "[check-tidb-ddl] Starting TiDB (if needed) and running AutoMigrate against it..."
"${COMPOSE[@]}" run --rm tidb-schema-check

echo "[check-tidb-ddl] OK - schema changes apply cleanly to TiDB."
echo "[check-tidb-ddl] (TiDB instance left running; './scripts/check-tidb-ddl.sh down' to remove it)"
