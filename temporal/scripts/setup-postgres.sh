#!/bin/sh
set -e

# temporal DB のスキーマをセットアップする
temporal-sql-tool \
  --plugin postgres12 \
  --ep "${POSTGRES_SEEDS}" \
  --port "${DB_PORT}" \
  --user "${POSTGRES_USER}" \
  --password "${POSTGRES_PWD}" \
  --db temporal \
  setup-schema \
  --version 0.0

temporal-sql-tool \
  --plugin postgres12 \
  --ep "${POSTGRES_SEEDS}" \
  --port "${DB_PORT}" \
  --user "${POSTGRES_USER}" \
  --password "${POSTGRES_PWD}" \
  --db temporal \
  update-schema \
  --schema-dir /etc/temporal/schema/postgresql/v12/temporal/versioned

# temporal_visibility DB のスキーマをセットアップする
temporal-sql-tool \
  --plugin postgres12 \
  --ep "${POSTGRES_SEEDS}" \
  --port "${DB_PORT}" \
  --user "${POSTGRES_USER}" \
  --password "${POSTGRES_PWD}" \
  --db temporal_visibility \
  setup-schema \
  --version 0.0

temporal-sql-tool \
  --plugin postgres12 \
  --ep "${POSTGRES_SEEDS}" \
  --port "${DB_PORT}" \
  --user "${POSTGRES_USER}" \
  --password "${POSTGRES_PWD}" \
  --db temporal_visibility \
  update-schema \
  --schema-dir /etc/temporal/schema/postgresql/v12/visibility/versioned
