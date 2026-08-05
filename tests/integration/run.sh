#!/usr/bin/env bash

set -Eeuo pipefail

readonly SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
readonly PROJECT_ROOT="$(cd -- "${SCRIPT_DIR}/../.." && pwd)"
cd "${PROJECT_ROOT}"

export INTEGRATION_MYSQL_PASSWORD="${INTEGRATION_MYSQL_PASSWORD:-courseforge-integration}"
export INTEGRATION_MYSQL_PORT="${INTEGRATION_MYSQL_PORT:-13306}"
export INTEGRATION_REDIS_PORT="${INTEGRATION_REDIS_PORT:-16379}"
export INTEGRATION_RABBITMQ_PORT="${INTEGRATION_RABBITMQ_PORT:-15673}"
export INTEGRATION_RABBITMQ_USER="${INTEGRATION_RABBITMQ_USER:-courseforge-integration}"
export INTEGRATION_RABBITMQ_PASSWORD="${INTEGRATION_RABBITMQ_PASSWORD:-courseforge-integration}"

readonly -a COMPOSE=(docker compose -f compose.integration.yaml)

cleanup() {
	"${COMPOSE[@]}" down --volumes --remove-orphans
}
trap cleanup EXIT

"${COMPOSE[@]}" up -d --wait mysql redis rabbitmq

expected_table_count="$(grep -c '^CREATE TABLE' docs/sql/courseforge.sql)"
actual_table_count="$(
	"${COMPOSE[@]}" exec -T mysql sh -ec \
		'mysql -uroot -p"$MYSQL_ROOT_PASSWORD" -Nse "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = '\''courseforge'\''"'
)"
if [[ "${actual_table_count}" != "${expected_table_count}" ]]; then
	echo "CourseForge integration schema mismatch: expected ${expected_table_count} tables, found ${actual_table_count}" >&2
	"${COMPOSE[@]}" exec -T mysql sh -ec \
		'mysql -uroot -p"$MYSQL_ROOT_PASSWORD" -Nse "SELECT table_name FROM information_schema.tables WHERE table_schema = '\''courseforge'\'' ORDER BY table_name"' >&2
	exit 1
fi

[[ "$("${COMPOSE[@]}" exec -T redis redis-cli ping)" == "PONG" ]]
"${COMPOSE[@]}" exec -T rabbitmq rabbitmq-diagnostics -q ping >/dev/null

COURSEFORGE_INTEGRATION_MYSQL_DSN="root:${INTEGRATION_MYSQL_PASSWORD}@tcp(127.0.0.1:${INTEGRATION_MYSQL_PORT})/courseforge?charset=utf8mb4&parseTime=True&loc=Local&timeout=5s" \
	COURSEFORGE_INTEGRATION_REDIS_ADDR="127.0.0.1:${INTEGRATION_REDIS_PORT}" \
	COURSEFORGE_INTEGRATION_RABBITMQ_ADDR="127.0.0.1:${INTEGRATION_RABBITMQ_PORT}" \
	COURSEFORGE_INTEGRATION_RABBITMQ_USER="${INTEGRATION_RABBITMQ_USER}" \
	COURSEFORGE_INTEGRATION_RABBITMQ_PASSWORD="${INTEGRATION_RABBITMQ_PASSWORD}" \
	go test -tags=integration ./tests/integration/... ./cmd/benchmark/enrollment -count=1
