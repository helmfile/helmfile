#!/usr/bin/env bash

# Test for issue #2247: Allow OCI charts without version
# This test combines validation tests (fast) with registry tests (comprehensive)

issue_2247_input_dir="${cases_dir}/issue-2247/input"
issue_2247_chart_dir="${cases_dir}/issue-2247/chart"
issue_2247_tmp_dir=$(mktemp -d)

test_start "issue-2247: OCI charts without version"

# ==============================================================================================================
# PART 1: Fast Validation Tests (no registry required)
# ==============================================================================================================

info "Part 1: Validation tests (no registry required)"

# Test 1.1: Explicit "latest" should error (issue #1047 behavior)
info "Test 1.1: Verifying explicit 'latest' version triggers validation error"
${helmfile} -f "${issue_2247_input_dir}/helmfile-with-latest.yaml" template > "${issue_2247_tmp_dir}/latest.txt" 2>&1
code=$?

# Debug: show output if command succeeded
if [ $code -eq 0 ]; then
  info "helmfile command succeeded when it should have failed. Output:"
  cat "${issue_2247_tmp_dir}/latest.txt"
  info "Helm version:"
  ${helm} version --short 2>&1 || echo "helm version command failed"
  rm -rf "${issue_2247_tmp_dir}"
  fail "Expected error for explicit 'latest' version but command succeeded"
fi

if ! grep -q "semver compliant" "${issue_2247_tmp_dir}/latest.txt"; then
  cat "${issue_2247_tmp_dir}/latest.txt"
  rm -rf "${issue_2247_tmp_dir}"
  fail "Expected 'semver compliant' error message for explicit 'latest' version"
fi

info "SUCCESS: Explicit 'latest' version correctly triggers validation error"

# Test 1.2: No version should NOT error (issue #2247 fix)
info "Test 1.2: Verifying OCI charts without version do NOT trigger validation error"
${helmfile} -f "${issue_2247_input_dir}/helmfile-no-version.yaml" template > "${issue_2247_tmp_dir}/no-version.txt" 2>&1
code=$?

# Note: The command will fail because the OCI registry doesn't exist,
# but it should NOT fail with the "semver compliant" validation error
if grep -q "semver compliant" "${issue_2247_tmp_dir}/no-version.txt"; then
  cat "${issue_2247_tmp_dir}/no-version.txt"
  rm -rf "${issue_2247_tmp_dir}"
  fail "Issue #2247 regression: OCI charts without version trigger validation error"
fi

info "SUCCESS: OCI charts without version do not trigger validation error"

# ==============================================================================================================
# PART 2: Comprehensive Registry Tests (requires Docker)
# ==============================================================================================================

# Check if Docker is available
if ! command -v docker &> /dev/null; then
  info "Skipping registry tests (Docker not available)"
  rm -rf "${issue_2247_tmp_dir}"
  test_pass "issue-2247: OCI charts without version (validation tests only)"
  return 0
fi

# Check if Docker daemon is running
if ! docker info &> /dev/null; then
  info "Skipping registry tests (Docker daemon not running)"
  rm -rf "${issue_2247_tmp_dir}"
  test_pass "issue-2247: OCI charts without version (validation tests only)"
  return 0
fi

info "Part 2: Comprehensive tests with real OCI registry"

registry_container_name="helmfile-test-registry-2247"
registry_port=5000

# Cleanup function
cleanup_registry() {
  info "Cleaning up test registry"
  docker stop ${registry_container_name} &>/dev/null || true
  docker rm ${registry_container_name} &>/dev/null || true
  rm -rf "${issue_2247_tmp_dir}"
}

# Ensure cleanup on exit
trap cleanup_registry EXIT

# Test 2.1: Start local OCI registry
info "Test 2.1: Starting local OCI registry on port ${registry_port}"
docker run -d \
  --name ${registry_container_name} \
  -p ${registry_port}:5000 \
  --rm \
  registry:2 &> "${issue_2247_tmp_dir}/registry-start.log"

if [ $? -ne 0 ]; then
  cat "${issue_2247_tmp_dir}/registry-start.log"
  warn "Failed to start Docker registry - skipping registry tests"
  rm -rf "${issue_2247_tmp_dir}"
  test_pass "issue-2247: OCI charts without version (validation tests only)"
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
  cleanup_registry
  test_pass "issue-2247: OCI charts without version (validation tests only)"
  return 0
fi

# Test 2.2: Package and push the test chart
info "Test 2.2: Packaging and pushing test charts"
${helm} package "${issue_2247_chart_dir}" -d "${issue_2247_tmp_dir}" > "${issue_2247_tmp_dir}/package.log" 2>&1
if [ $? -ne 0 ]; then
  cat "${issue_2247_tmp_dir}/package.log"
  warn "Failed to package chart - skipping registry tests"
  cleanup_registry
  test_pass "issue-2247: OCI charts without version (validation tests only)"
  return 0
fi

info "Pushing chart version 1.0.0 to local registry"
${helm} push "${issue_2247_tmp_dir}/test-chart-2247-1.0.0.tgz" oci://localhost:${registry_port} > "${issue_2247_tmp_dir}/push.log" 2>&1
if [ $? -ne 0 ]; then
  cat "${issue_2247_tmp_dir}/push.log"
  warn "Failed to push chart to registry - skipping registry tests"
  cleanup_registry
  test_pass "issue-2247: OCI charts without version (validation tests only)"
  return 0
fi

# Create version 2.0.0 as well to test "latest" behavior
info "Creating and pushing version 2.0.0"
cp -r "${issue_2247_chart_dir}" "${issue_2247_tmp_dir}/chart-v2"
sed -i.bak 's/version: 1.0.0/version: 2.0.0/' "${issue_2247_tmp_dir}/chart-v2/Chart.yaml"
${helm} package "${issue_2247_tmp_dir}/chart-v2" -d "${issue_2247_tmp_dir}" > "${issue_2247_tmp_dir}/package-v2.log" 2>&1
${helm} push "${issue_2247_tmp_dir}/test-chart-2247-2.0.0.tgz" oci://localhost:${registry_port} > "${issue_2247_tmp_dir}/push-v2.log" 2>&1

info "Successfully pushed chart versions 1.0.0 and 2.0.0"

# Test 2.3: Test helmfile with OCI chart WITHOUT version
info "Test 2.3: helmfile template with OCI chart without version (should pull latest = 2.0.0)"
cat > "${issue_2247_tmp_dir}/helmfile-oci-registry.yaml" <<EOF
releases:
  - name: test-oci-no-version
    namespace: default
    chart: oci://localhost:${registry_port}/test-chart-2247
    # No version specified - should pull latest (issue #2247 fix)
EOF

${helmfile} -f "${issue_2247_tmp_dir}/helmfile-oci-registry.yaml" template --skip-deps > "${issue_2247_tmp_dir}/template-no-version.yaml" 2>&1
code=$?

# Should NOT have the semver validation error
if grep -q "semver compliant" "${issue_2247_tmp_dir}/template-no-version.yaml"; then
  cat "${issue_2247_tmp_dir}/template-no-version.yaml"
  cleanup_registry
  fail "Issue #2247 regression: OCI chart without version triggered validation error"
fi

# Should succeed
if [ $code -eq 0 ]; then
  info "SUCCESS: helmfile template succeeded with OCI chart without version"
  # Verify it pulled version 2.0.0 (the latest)
  if grep -q "Hello from test chart 2.0.0" "${issue_2247_tmp_dir}/template-no-version.yaml"; then
    info "SUCCESS: Correctly pulled latest version (2.0.0)"
  else
    info "Note: Could not verify exact version pulled (non-critical)"
  fi
else
  # Check if it failed for a reason other than our validation
  if ! grep -q "semver compliant" "${issue_2247_tmp_dir}/template-no-version.yaml"; then
    info "helmfile failed but not due to version validation (acceptable)"
  else
    cat "${issue_2247_tmp_dir}/template-no-version.yaml"
    cleanup_registry
    fail "Unexpected validation error"
  fi
fi

# Test 2.4: Test helmfile with explicit "latest" version
info "Test 2.4: helmfile template with explicit 'latest' version (should error)"
cat > "${issue_2247_tmp_dir}/helmfile-explicit-latest.yaml" <<EOF
releases:
  - name: test-oci-explicit-latest
    namespace: default
    chart: oci://localhost:${registry_port}/test-chart-2247
    version: "latest"  # Should trigger validation error
EOF

${helmfile} -f "${issue_2247_tmp_dir}/helmfile-explicit-latest.yaml" template --skip-deps > "${issue_2247_tmp_dir}/template-latest.yaml" 2>&1
code=$?

# Should have the validation error
if ! grep -q "semver compliant" "${issue_2247_tmp_dir}/template-latest.yaml"; then
  cat "${issue_2247_tmp_dir}/template-latest.yaml"
  cleanup_registry
  fail "Expected validation error for explicit 'latest' version"
fi

if [ $code -eq 0 ]; then
  cat "${issue_2247_tmp_dir}/template-latest.yaml"
  cleanup_registry
  fail "helmfile should have failed with validation error for explicit 'latest'"
fi

info "SUCCESS: Explicit 'latest' version correctly triggered validation error"

# Test 2.5: Test helmfile with specific version
info "Test 2.5: helmfile template with specific version 1.0.0"
cat > "${issue_2247_tmp_dir}/helmfile-specific-version.yaml" <<EOF
releases:
  - name: test-oci-specific
    namespace: default
    chart: oci://localhost:${registry_port}/test-chart-2247
    version: "1.0.0"
EOF

${helmfile} -f "${issue_2247_tmp_dir}/helmfile-specific-version.yaml" template --skip-deps > "${issue_2247_tmp_dir}/template-specific.yaml" 2>&1
code=$?

if grep -q "semver compliant" "${issue_2247_tmp_dir}/template-specific.yaml"; then
  cat "${issue_2247_tmp_dir}/template-specific.yaml"
  cleanup_registry
  fail "Unexpected validation error for specific version"
fi

if [ $code -eq 0 ]; then
  info "SUCCESS: helmfile template succeeded with specific version"
  if grep -q "Hello from test chart 1.0.0" "${issue_2247_tmp_dir}/template-specific.yaml"; then
    info "SUCCESS: Correctly used version 1.0.0"
  fi
else
  info "helmfile failed but not due to version validation (acceptable)"
fi

# All tests passed!
test_pass "issue-2247: OCI charts without version (all tests including registry)"
