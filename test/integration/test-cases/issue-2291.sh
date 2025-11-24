#!/usr/bin/env bash

# Test for issue #2291: strategicMergePatches should NOT cause CRDs to be removed
# Issue: https://github.com/helmfile/helmfile/issues/2291
#
# Problem: When using strategicMergePatches, chartify was relocating CRDs from
# templates/crds/ to the root crds/ directory, changing how Helm manages them.
# This caused helm diff to incorrectly show CRDs as being removed.
#
# Fix: Chartify now preserves the original CRD location (templates/crds/)

issue_2291_input_dir="${cases_dir}/issue-2291/input"
issue_2291_tmp_dir=$(mktemp -d)

test_start "issue-2291: CRDs preserved with strategicMergePatches"

# Test 1: Template the chart to verify CRDs are included
info "Step 1: Templating chart to verify CRD structure"
${helmfile} -f "${issue_2291_input_dir}/helmfile.yaml" template > "${issue_2291_tmp_dir}/templated.yaml" 2>&1
code=$?

if [ $code -ne 0 ]; then
  cat "${issue_2291_tmp_dir}/templated.yaml"
  rm -rf "${issue_2291_tmp_dir}"
  fail "Failed to template chart"
fi

# Verify CRD is in the templated output
if ! grep -q "kind: CustomResourceDefinition" "${issue_2291_tmp_dir}/templated.yaml"; then
  cat "${issue_2291_tmp_dir}/templated.yaml"
  rm -rf "${issue_2291_tmp_dir}"
  fail "CRD not found in templated output"
fi

info "✓ CRD found in templated output"

# Verify the CRD name
if ! grep -q "name: testresources.test.io" "${issue_2291_tmp_dir}/templated.yaml"; then
  cat "${issue_2291_tmp_dir}/templated.yaml"
  rm -rf "${issue_2291_tmp_dir}"
  fail "Expected CRD 'testresources.test.io' not found"
fi

info "✓ CRD testresources.test.io found"

# Test 2: Apply the chart with strategicMergePatches
info "Step 2: Applying chart with strategicMergePatches"
${helmfile} -f "${issue_2291_input_dir}/helmfile.yaml" apply --suppress-diff > "${issue_2291_tmp_dir}/apply.txt" 2>&1
code=$?

if [ $code -ne 0 ]; then
  cat "${issue_2291_tmp_dir}/apply.txt"
  rm -rf "${issue_2291_tmp_dir}"
  fail "Failed to apply chart"
fi

info "✓ Chart applied successfully"

# Test 3: Verify CRD was created
info "Step 3: Verifying CRD was installed"
if ! ${kubectl} get crd testresources.test.io > /dev/null 2>&1; then
  rm -rf "${issue_2291_tmp_dir}"
  fail "CRD testresources.test.io was not installed"
fi

info "✓ CRD testresources.test.io is installed"

# Test 4: Run diff - should show NO changes (especially NO CRD removal)
info "Step 4: Running diff - should show no CRD removal"
${helmfile} -f "${issue_2291_input_dir}/helmfile.yaml" diff > "${issue_2291_tmp_dir}/diff.txt" 2>&1
diff_code=$?

# Check if diff shows CRD being removed (the bug we're fixing)
if grep -q "testresources.test.io.*will be deleted" "${issue_2291_tmp_dir}/diff.txt" || \
  grep -q "testresources.test.io.*removed" "${issue_2291_tmp_dir}/diff.txt" || \
  grep -q -- "- kind: CustomResourceDefinition" "${issue_2291_tmp_dir}/diff.txt"; then
  cat "${issue_2291_tmp_dir}/diff.txt"
  rm -rf "${issue_2291_tmp_dir}"
  fail "BUG DETECTED: helm diff shows CRD being removed (issue #2291 regression)"
fi

info "✓ CRD is NOT marked for removal"

# Test 5: Verify Deployment has the DNS patch applied
info "Step 5: Verifying strategic merge patch was applied to Deployment"
if ! ${kubectl} get deployment test-app -o yaml | grep -q "ndots"; then
  rm -rf "${issue_2291_tmp_dir}"
  fail "DNS patch was not applied to Deployment"
fi

info "✓ Strategic merge patch applied successfully"

# Cleanup
info "Cleaning up"
${helm} uninstall test-release-2291 2>/dev/null || true
${kubectl} delete crd testresources.test.io 2>/dev/null || true
rm -rf "${issue_2291_tmp_dir}"

test_pass "issue-2291: CRDs preserved with strategicMergePatches"
