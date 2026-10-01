#!/bin/bash
set -euo pipefail

cd "$(dirname "${BASH_SOURCE[0]}")/.."

export GOCACHE=$(mktemp -d /tmp/gocache.XXXXXX)
export GOMODCACHE=$(mktemp -d /tmp/gomodcache.XXXXXX)
export GOFLAGS=-mod=mod

coverage_mode=""
case "${1:-}" in
    "") ;;
    --coverage) coverage_mode=1 ;;
    --coverage=html|--html) coverage_mode=html ;;
    *)
        echo "Usage: $0 [--coverage|--coverage=html]" >&2
        exit 2
        ;;
esac

if [[ -n "$coverage_mode" ]]; then
    make test-unit COVERAGE="$coverage_mode"
else
    make test-unit
fi

if [[ -n "${ARTIFACT_DIR:-}" ]]; then
    cp -r coverage-*.out coverage-*.html "${ARTIFACT_DIR}/" 2>/dev/null || true
fi
