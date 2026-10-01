#!/bin/bash
# Upload unit-test coverage for each Go module to Codecov.

set -euo pipefail

cd "$(dirname "${BASH_SOURCE[0]}")/.."

CODECOV_TOKEN=$(< /var/run/codecov-secret/CODECOV_TOKEN)
CODECOV_ENTERPRISE_URL=$(< /var/run/codecov-secret/CODECOV_ENTERPRISE_URL)

trap 'rm -f codecov codecov.SHA256SUM coverage-*.out coverage-*.html' EXIT

./ci/unit-tests.sh --coverage

GIT_COMMIT="${PULL_PULL_SHA:-${PULL_BASE_SHA:-$(git rev-parse HEAD)}}"
GIT_BRANCH="${PULL_HEAD_REF:-${PULL_BASE_REF:-$(git rev-parse --abbrev-ref HEAD)}}"

CODECOV_VERSION="v11.3.1"
curl -fsSO --connect-timeout 10 --max-time 60 --retry 3 --retry-max-time 90 \
    "https://cli.codecov.io/${CODECOV_VERSION}/linux/codecov"
curl -fsSO --connect-timeout 10 --max-time 60 --retry 3 --retry-max-time 90 \
    "https://cli.codecov.io/${CODECOV_VERSION}/linux/codecov.SHA256SUM"
sha256sum --check codecov.SHA256SUM
chmod +x codecov

UPLOAD_ARGS=(
    --enterprise-url "${CODECOV_ENTERPRISE_URL}"
    upload-process
    --fail-on-error
    --slug="openshift-online/rosa-hyperfleet-api"
    --git-service github
    --commit-sha "${GIT_COMMIT}"
    --branch "${GIT_BRANCH}"
    --disable-search
)

if [[ -n "${PULL_NUMBER:-}" ]]; then
    UPLOAD_ARGS+=(--pr "${PULL_NUMBER}")
    UPLOAD_ARGS+=(--parent-sha "${PULL_BASE_SHA}")
fi

for coverage in \
    "platform-api:coverage-platform-api.out" \
    "hyperfleet-operator:coverage-hyperfleet-operator.out" \
    "api-codegen:coverage-api-codegen.out" \
    "clientset:coverage-clientset.out"; do
    flag="${coverage%%:*}"
    file="${coverage#*:}"

    if [[ ! -f "$file" ]]; then
        echo "Error: coverage profile not found: $file" >&2
        exit 1
    fi

    echo "Uploading $file with flag $flag"
    CODECOV_TOKEN="${CODECOV_TOKEN}" ./codecov "${UPLOAD_ARGS[@]}" \
        --flag "${flag}" \
        --file "${file}"
done

echo "Coverage upload complete."
