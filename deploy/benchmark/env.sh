#!/usr/bin/env bash

set -Eeuo pipefail

readonly SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
readonly COMPOSE_FILE="${SCRIPT_DIR}/compose.yaml"
readonly -a COMPOSE=(docker compose -f "${COMPOSE_FILE}")

case "${1:-}" in
	up)
		"${COMPOSE[@]}" up -d --wait mysql redis rabbitmq
		"${COMPOSE[@]}" up -d api
		if ! docker run --rm --network courseforge-benchmark_backend busybox:1.37 \
			/bin/sh -ec '
				for attempt in $(seq 1 60); do
					wget -q -T 2 -O /dev/null http://api:8080/readyz && exit 0
					sleep 2
				done
				exit 1
			'; then
			echo "CourseForge API did not become ready" >&2
			"${COMPOSE[@]}" logs --tail=100 api >&2
			exit 1
		fi
		echo "CourseForge benchmark environment is ready"
		;;
	down)
		"${COMPOSE[@]}" down --volumes --remove-orphans
		;;
	*)
		echo "Usage: ./deploy/benchmark/env.sh up|down" >&2
		exit 2
		;;
esac
