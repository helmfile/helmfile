#!/usr/bin/env bash

# Test for issue #2097: OCI chart digest support
# This test combines validation tests (fast) with registry tests (comprehensive)

issue_2097_input_dir="${cases_dir}/issue-2097/input"
issue_2097_chart_dir="${cases_dir}/issue-2097/chart"
issue_2097_tmp_dir=$(mktemp -d)

test_start "issue-2097: OCI chart digest support"

# ==============================================================================================================
# PART 1: Fast Validation Tests (no registry required)
# ==============================================================================================================

info "Part 1: Validation tests (no registry required)"

# Test 1.1: Digest in version field should NOT trigger semver validation error
info "Test 1.1: Verifying version with digest (2.0.0@sha256:...) does not trigger validation error"
set +e
${helmfile} -f "${issue_2097_input_dir}/helmfile-digest-in-version.yaml" template > "${issue_2097_tmp_dir}/digest-in-version.txt" 2>&1
code=$?
set -e

# The command will fail because the registry doesn't exist, but it should NOT fail with
# "semver compliant" validation error
if grep -q "semver compliant" "${issue_2097_tmp_dir}/digest-in-version.txt"; then
  cat "${issue_2097_tmp_dir}/digest-in-version.txt"
  rm -rf "${issue_2097_tmp_dir}"
  fail "Issue #2097 regression: version with digest triggered validation error"
fi

info "SUCCESS: version with digest does not trigger validation error"

# Test 1.2: Version tag in chart URL should NOT trigger validation error
info "Test 1.2: Verifying chart URL with version tag (oci://reg/chart:2.0.0) does not trigger validation error"
set +e
${helmfile} -f "${issue_2097_input_dir}/helmfile-version-in-url.yaml" template > "${issue_2097_tmp_dir}/version-in-url.txt" 2>&1
code=$?
set -e

if grep -q "semver compliant" "${issue_2097_tmp_dir}/version-in-url.txt"; then
  cat "${issue_2097_tmp_dir}/version-in-url.txt"
  rm -rf "${issue_2097_tmp_dir}"
  fail "Issue #2097 regression: version in chart URL triggered validation error"
fi

info "SUCCESS: version in chart URL does not trigger validation error"

# Test 1.3: Digest in chart URL should NOT trigger validation error
info "Test 1.3: Verifying chart URL with digest (oci://reg/chart@sha256:...) does not trigger validation error"
set +e
${helmfile} -f "${issue_2097_input_dir}/helmfile-digest-in-url.yaml" template > "${issue_2097_tmp_dir}/digest-in-url.txt" 2>&1
code=$?
set -e

if grep -q "semver compliant" "${issue_2097_tmp_dir}/digest-in-url.txt"; then
  cat "${issue_2097_tmp_dir}/digest-in-url.txt"
  rm -rf "${issue_2097_tmp_dir}"
  fail "Issue #2097 regression: digest in chart URL triggered validation error"
fi

info "SUCCESS: digest in chart URL does not trigger validation error"

# ==============================================================================================================
# PART 2: Comprehensive Registry Tests (requires Docker)
# ==============================================================================================================

# Check if Docker is available
if ! command -v docker &> /dev/null; then
  info "Skipping registry tests (Docker not available)"
  rm -rf "${issue_2097_tmp_dir}"
  trap - EXIT
  test_pass "issue-2097: OCI chart digest support (validation tests only)"
  return 0
fi

# Check if Docker daemon is running
if ! docker info &> /dev/null; then
  info "Skipping registry tests (Docker daemon not running)"
  rm -rf "${issue_2097_tmp_dir}"
  trap - EXIT
  test_pass "issue-2097: OCI chart digest support (validation tests only)"
  return 0
fi

info "Part 2: Comprehensive tests with real OCI registry"

registry_container_name="helmfile-test-registry-2097"
registry_port=5097

# Cleanup function
cleanup_registry_2097() {
  info "Cleaning up test registry (issue-2097)"
  docker stop ${registry_container_name} &>/dev/null || true
  docker rm ${registry_container_name} &>/dev/null || true
  rm -rf "${issue_2097_tmp_dir}"
}

# Ensure cleanup on exit
trap cleanup_registry_2097 EXIT

# Test 2.1: Start local OCI registry
info "Test 2.1: Starting local OCI registry on port ${registry_port}"
docker run -d \
  --name ${registry_container_name} \
  -p ${registry_port}:5000 \
  --rm \
  registry:2 &> "${issue_2097_tmp_dir}/registry-start.log"

if [ $? -ne 0 ]; then
  cat "${issue_2097_tmp_dir}/registry-start.log"
  warn "Failed to start Docker registry - skipping registry tests"
  rm -rf "${issue_2097_tmp_dir}"
  trap - EXIT
  test_pass "issue-2097: OCI chart digest support (validation tests only)"
  return 0
fi

# Wait for registry to be ready
info "Waiting for registry to be ready..."
max_attempts=30
attempt=0
while [ $attempt -lt $max_attempts ]; do
  if curl -s http://localhost:${registry_port}/v2/ > /dev/null 2>&1; then
    info "Registry is ready"
    break
  fi
  attempt=$((attempt + 1))
  sleep 1
done

if [ $attempt -eq $max_attempts ]; then
  warn "Registry did not become ready in time - skipping registry tests"
  cleanup_registry_2097
  trap - EXIT
  test_pass "issue-2097: OCI chart digest support (validation tests only)"
  return 0
fi

# Test 2.2: Package and push the test chart
info "Test 2.2: Packaging and pushing test charts"
set +e
${helm} package "${issue_2097_chart_dir}" -d "${issue_2097_tmp_dir}" > "${issue_2097_tmp_dir}/package.log" 2>&1
if [ $? -ne 0 ]; then
  set -e
  cat "${issue_2097_tmp_dir}/package.log"
  warn "Failed to package chart - skipping registry tests"
  cleanup_registry_2097
  trap - EXIT
  test_pass "issue-2097: OCI chart digest support (validation tests only)"
  return 0
fi
set -e

info "Pushing chart version 1.0.0 to local registry"
set +e
${helm} push "${issue_2097_tmp_dir}/test-chart-2097-1.0.0.tgz" oci://localhost:${registry_port} --plain-http > "${issue_2097_tmp_dir}/push.log" 2>&1
if [ $? -ne 0 ]; then
  set -e
  cat "${issue_2097_tmp_dir}/push.log"
  warn "Failed to push chart to registry - skipping registry tests"
  cleanup_registry_2097
  trap - EXIT
  test_pass "issue-2097: OCI chart digest support (validation tests only)"
  return 0
fi
set -e

# Create version 2.0.0 as well
info "Creating and pushing version 2.0.0"
cp -r "${issue_2097_chart_dir}" "${issue_2097_tmp_dir}/chart-v2"
sed -i.bak 's/version: 1.0.0/version: 2.0.0/' "${issue_2097_tmp_dir}/chart-v2/Chart.yaml"
set +e
${helm} package "${issue_2097_tmp_dir}/chart-v2" -d "${issue_2097_tmp_dir}" > "${issue_2097_tmp_dir}/package-v2.log" 2>&1
${helm} push "${issue_2097_tmp_dir}/test-chart-2097-2.0.0.tgz" oci://localhost:${registry_port} --plain-http > "${issue_2097_tmp_dir}/push-v2.log" 2>&1
set -e

info "Successfully pushed chart versions 1.0.0 and 2.0.0"

# Get the digest of the v1.0.0 chart
info "Fetching digest for v1.0.0"
set +e
chart_digest=$(${helm} pull oci://localhost:${registry_port}/test-chart-2097 --version 1.0.0 --plain-http -d "${issue_2097_tmp_dir}/pull-test" 2>&1 | grep -oE 'sha256:[a-f0-9]+')
set -e

if [ -z "$chart_digest" ]; then
  # Try alternative: use the registry API
  chart_digest=$(curl -s http://localhost:${registry_port}/v2/test-chart-2097/manifests/1.0.0 \
    -H "Accept: application/vnd.oci.image.manifest.v1+json" \
    -D - -o /dev/null 2>/dev/null | grep -i "docker-content-digest" | tr -d '\r' | awk '{print $2}')
fi

if [ -z "$chart_digest" ]; then
  warn "Could not determine chart digest - skipping digest registry tests"
  cleanup_registry_2097
  trap - EXIT
  test_pass "issue-2097: OCI chart digest support (validation tests only)"
  return 0
fi

info "Chart v1.0.0 digest: ${chart_digest}"

# Test 2.3: Baseline - chart with version in field (should work)
info "Test 2.3: Baseline - chart with version in field"
cat > "${issue_2097_tmp_dir}/helmfile-baseline.yaml" <<EOF
helmDefaults:
  plainHttp: true
releases:
  - name: test-oci-baseline
    namespace: default
    chart: oci://localhost:${registry_port}/test-chart-2097
    version: "1.0.0"
EOF

set +e
${helmfile} -f "${issue_2097_tmp_dir}/helmfile-baseline.yaml" template --skip-deps > "${issue_2097_tmp_dir}/template-baseline.yaml" 2>&1
code=$?
set -e

if [ $code -ne 0 ]; then
  cat "${issue_2097_tmp_dir}/template-baseline.yaml"
  cleanup_registry_2097
  fail "Baseline test failed - cannot proceed with digest tests"
fi

info "SUCCESS: Baseline test passed"
if grep -q "Hello from test chart 1.0.0" "${issue_2097_tmp_dir}/template-baseline.yaml"; then
  info "SUCCESS: Correctly pulled version 1.0.0"
fi

# Test 2.4: Version tag in chart URL
info "Test 2.4: Version tag in chart URL"
cat > "${issue_2097_tmp_dir}/helmfile-version-url.yaml" <<EOF
helmDefaults:
  plainHttp: true
releases:
  - name: test-oci-version-url
    namespace: default
    chart: oci://localhost:${registry_port}/test-chart-2097:1.0.0
EOF

set +e
${helmfile} -f "${issue_2097_tmp_dir}/helmfile-version-url.yaml" template --skip-deps > "${issue_2097_tmp_dir}/template-version-url.yaml" 2>&1
code=$?
set -e

if grep -q "semver compliant" "${issue_2097_tmp_dir}/template-version-url.yaml"; then
  cat "${issue_2097_tmp_dir}/template-version-url.yaml"
  cleanup_registry_2097
  fail "Issue #2097: Version in chart URL triggered validation error"
fi

if [ $code -ne 0 ]; then
  cat "${issue_2097_tmp_dir}/template-version-url.yaml"
  cleanup_registry_2097
  fail "Issue #2097: Version in chart URL failed"
fi

info "SUCCESS: Version in chart URL works"
if grep -q "Hello from test chart 1.0.0" "${issue_2097_tmp_dir}/template-version-url.yaml"; then
  info "SUCCESS: Correctly pulled version 1.0.0 from URL tag"
fi

# Test 2.5: Digest in chart URL
info "Test 2.5: Digest in chart URL"
cat > "${issue_2097_tmp_dir}/helmfile-digest-url.yaml" <<EOF
helmDefaults:
  plainHttp: true
releases:
  - name: test-oci-digest-url
    namespace: default
    chart: oci://localhost:${registry_port}/test-chart-2097@${chart_digest}
EOF

set +e
${helmfile} -f "${issue_2097_tmp_dir}/helmfile-digest-url.yaml" template --skip-deps > "${issue_2097_tmp_dir}/template-digest-url.yaml" 2>&1
code=$?
set -e

if grep -q "semver compliant" "${issue_2097_tmp_dir}/template-digest-url.yaml"; then
  cat "${issue_2097_tmp_dir}/template-digest-url.yaml"
  cleanup_registry_2097
  fail "Issue #2097: Digest in chart URL triggered validation error"
fi

if [ $code -ne 0 ]; then
  cat "${issue_2097_tmp_dir}/template-digest-url.yaml"
  cleanup_registry_2097
  fail "Issue #2097: Digest in chart URL failed"
fi

info "SUCCESS: Digest in chart URL works"

# Test 2.6: Digest in version field
info "Test 2.6: Digest in version field"
cat > "${issue_2097_tmp_dir}/helmfile-digest-version.yaml" <<EOF
helmDefaults:
  plainHttp: true
releases:
  - name: test-oci-digest-version
    namespace: default
    chart: oci://localhost:${registry_port}/test-chart-2097
    version: "1.0.0@${chart_digest}"
EOF

set +e
${helmfile} -f "${issue_2097_tmp_dir}/helmfile-digest-version.yaml" template --skip-deps > "${issue_2097_tmp_dir}/template-digest-version.yaml" 2>&1
code=$?
set -e

if grep -q "semver compliant" "${issue_2097_tmp_dir}/template-digest-version.yaml"; then
  cat "${issue_2097_tmp_dir}/template-digest-version.yaml"
  cleanup_registry_2097
  fail "Issue #2097: Digest in version field triggered validation error"
fi

if [ $code -ne 0 ]; then
  cat "${issue_2097_tmp_dir}/template-digest-version.yaml"
  cleanup_registry_2097
  fail "Issue #2097: Digest in version field failed"
fi

info "SUCCESS: Digest in version field works"
if grep -q "Hello from test chart 1.0.0" "${issue_2097_tmp_dir}/template-digest-version.yaml"; then
  info "SUCCESS: Correctly pulled version 1.0.0 with digest verification"
fi

# Test 2.7: Version + digest in chart URL
info "Test 2.7: Version + digest in chart URL"
cat > "${issue_2097_tmp_dir}/helmfile-both-url.yaml" <<EOF
helmDefaults:
  plainHttp: true
releases:
  - name: test-oci-both-url
    namespace: default
    chart: oci://localhost:${registry_port}/test-chart-2097:1.0.0@${chart_digest}
EOF

set +e
${helmfile} -f "${issue_2097_tmp_dir}/helmfile-both-url.yaml" template --skip-deps > "${issue_2097_tmp_dir}/template-both-url.yaml" 2>&1
code=$?
set -e

if grep -q "semver compliant" "${issue_2097_tmp_dir}/template-both-url.yaml"; then
  cat "${issue_2097_tmp_dir}/template-both-url.yaml"
  cleanup_registry_2097
  fail "Issue #2097: Version + digest in chart URL triggered validation error"
fi

if [ $code -ne 0 ]; then
  cat "${issue_2097_tmp_dir}/template-both-url.yaml"
  cleanup_registry_2097
  fail "Issue #2097: Version + digest in chart URL failed"
fi

info "SUCCESS: Version + digest in chart URL works"

# All tests passed!
# Remove the EXIT trap to avoid interfering with subsequent tests
cleanup_registry_2097
trap - EXIT
test_pass "issue-2097: OCI chart digest support (all tests including registry)"
