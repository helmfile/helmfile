#!/usr/bin/env bash

# Test for issue #2271: lookup function should work with strategicMergePatches and jsonPatches
# Without this fix, helm template runs client-side and lookup() returns empty values

issue_2271_input_dir="${cases_dir}/issue-2271/input"
issue_2271_tmp_dir=$(mktemp -d)

cd "${issue_2271_input_dir}"

test_start "issue-2271: lookup function with strategicMergePatches and jsonPatches"

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

assert_lookup_preserved() {
  local label="$1"
  local helmfile_path="$2"
  local output_path="$3"

  info "Testing diff with ${label} - lookup should preserve value"

  ${helmfile} -f "${helmfile_path}" diff > "${output_path}" 2>&1
  code=$?

  # Check if the diff contains the preserved value (not "initial-value")
  if grep -q "preserved-value.*test-preserved-value" "${output_path}"; then
    info "SUCCESS: lookup function preserved the value with ${label}"
  elif grep -q "preserved-value.*initial-value" "${output_path}"; then
    cat "${output_path}"
    rm -rf "${issue_2271_tmp_dir}"
    fail "Issue #2271 regression: lookup function returned empty value with ${label}"
  else
    # No diff for ConfigMap means value is perfectly preserved
    info "SUCCESS: No ConfigMap changes detected for ${label} (value perfectly preserved)"
  fi
}

# Test 3: Diff with strategicMergePatches should preserve the lookup value
assert_lookup_preserved "strategicMergePatches" "helmfile.yaml" "${issue_2271_tmp_dir}/test-2271-strategic-diff.txt"

# Test 4: Diff with jsonPatches should preserve the lookup value
assert_lookup_preserved "jsonPatches" "helmfile-jsonpatch.yaml" "${issue_2271_tmp_dir}/test-2271-json-diff.txt"

# Cleanup
${helm} uninstall test-release-2271 --namespace default 2>/dev/null || true
rm -rf "${issue_2271_tmp_dir}"

test_pass "issue-2271: lookup function with strategicMergePatches and jsonPatches"
