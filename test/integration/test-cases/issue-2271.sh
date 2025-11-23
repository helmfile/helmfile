#!/usr/bin/env bash

# Test for issue #2271: lookup function should work with strategicMergePatches
# Without this fix, helm template runs client-side and lookup() returns empty values

issue_2271_input_dir="${cases_dir}/issue-2271/input"
issue_2271_tmp_dir=$(mktemp -d)

cd "${issue_2271_input_dir}"

test_start "issue-2271: lookup function with strategicMergePatches"

# Test 1: Install chart without kustomize patches
info "Installing chart without kustomize patches"
${helmfile} -f helmfile-no-kustomize.yaml apply --suppress-diff > "${issue_2271_tmp_dir}/test-2271-install.txt" 2>&1
code=$?

if [ $code -ne 0 ]; then
  cat "${issue_2271_tmp_dir}/test-2271-install.txt"
  rm -rf "${issue_2271_tmp_dir}"
  fail "Failed to install chart"
fi

info "Chart installed successfully"

# Test 2: Modify ConfigMap value manually to simulate an upgrade scenario
info "Modifying ConfigMap value to test lookup preservation"
${kubectl} patch configmap test-release-2271-config --type merge -p '{"data":{"preserved-value":"test-preserved-value"}}' > /dev/null 2>&1

# Verify the value was changed
current_value=$(${kubectl} get configmap test-release-2271-config -o jsonpath='{.data.preserved-value}')
if [ "$current_value" != "test-preserved-value" ]; then
  rm -rf "${issue_2271_tmp_dir}"
  fail "Failed to update ConfigMap value. Got: $current_value"
fi

info "ConfigMap value updated to: $current_value"

# Test 3: Diff with strategicMergePatches should preserve the lookup value
info "Testing diff with strategicMergePatches - lookup should preserve value"

${helmfile} -f helmfile.yaml diff > "${issue_2271_tmp_dir}/test-2271-diff.txt" 2>&1
code=$?

# Check if the diff contains the preserved value (not "initial-value")
if grep -q "preserved-value.*test-preserved-value" "${issue_2271_tmp_dir}/test-2271-diff.txt"; then
  info "SUCCESS: lookup function preserved the value with kustomize patches"
elif grep -q "preserved-value.*initial-value" "${issue_2271_tmp_dir}/test-2271-diff.txt"; then
  cat "${issue_2271_tmp_dir}/test-2271-diff.txt"
  rm -rf "${issue_2271_tmp_dir}"
  fail "Issue #2271 regression: lookup function returned empty value with kustomize"
else
  # No diff for ConfigMap means value is perfectly preserved
  info "SUCCESS: No ConfigMap changes detected (value perfectly preserved)"
fi

# Cleanup
${helm} uninstall test-release-2271 --namespace default 2>/dev/null || true
rm -rf "${issue_2271_tmp_dir}"

test_pass "issue-2271: lookup function with strategicMergePatches"
