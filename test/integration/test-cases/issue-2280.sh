#!/usr/bin/env bash

# Test for issue #2280: --color flag conflict with Helm 4
# In Helm 4, the --color flag is parsed by Helm before reaching the helm-diff plugin
# The fix removes --color/--no-color flags and uses HELM_DIFF_COLOR env var instead

# Only run this test on Helm 4
if [ "${HELMFILE_HELM4}" != "1" ]; then
  info "Skipping issue-2280 test (Helm 4 only)"
  return 0
fi

issue_2280_input_dir="${cases_dir}/issue-2280/input"
issue_2280_tmp_dir=$(mktemp -d)

cd "${issue_2280_input_dir}"

test_start "issue-2280: --color flag with Helm 4"

# Test 1: Install the chart first
info "Installing chart for issue #2280 test"
${helmfile} -f helmfile.yaml apply --suppress-diff > "${issue_2280_tmp_dir}/install.txt" 2>&1
code=$?

if [ $code -ne 0 ]; then
  cat "${issue_2280_tmp_dir}/install.txt"
  rm -rf "${issue_2280_tmp_dir}"
  fail "Failed to install chart"
fi

info "Chart installed successfully"

# Test 2: Run diff with --color and --context flags
# This is the exact scenario from issue #2280
# Before the fix, --color flag would be parsed by Helm 4 before reaching helm-diff plugin,
# consuming --context as its value, resulting in error: invalid color mode "--context"
# After the fix, --color is removed and HELM_DIFF_COLOR env var is set instead
info "Running diff with --color and --context flags"

${helmfile} -f helmfile.yaml diff --color --context 3 > "${issue_2280_tmp_dir}/diff-color.txt" 2>&1
code=$?

# Check for the error from issue #2280
if grep -q "invalid color mode" "${issue_2280_tmp_dir}/diff-color.txt"; then
  cat "${issue_2280_tmp_dir}/diff-color.txt"
  rm -rf "${issue_2280_tmp_dir}"
  fail "Issue #2280 regression: --color flag consumed --context argument"
fi

# diff command should succeed (exit code 0 or 2 with --detailed-exitcode)
if [ $code -ne 0 ]; then
  # Check if it's a diff-related error (not the color mode error)
  if ! grep -q "Comparing release" "${issue_2280_tmp_dir}/diff-color.txt"; then
    cat "${issue_2280_tmp_dir}/diff-color.txt"
    rm -rf "${issue_2280_tmp_dir}"
    fail "Diff command failed unexpectedly"
  fi
fi

info "SUCCESS: --color flag did not interfere with --context flag"

# Test 3: Also test with --no-color
info "Running diff with --no-color and --context flags"

${helmfile} -f helmfile.yaml diff --no-color --context 3 > "${issue_2280_tmp_dir}/diff-no-color.txt" 2>&1
code=$?

if grep -q "invalid color mode" "${issue_2280_tmp_dir}/diff-no-color.txt"; then
  cat "${issue_2280_tmp_dir}/diff-no-color.txt"
  rm -rf "${issue_2280_tmp_dir}"
  fail "Issue #2280 regression: --no-color flag consumed --context argument"
fi

info "SUCCESS: --no-color flag did not interfere with --context flag"

# Cleanup
${helm} uninstall test-release-2280 --namespace default 2>/dev/null || true
rm -rf "${issue_2280_tmp_dir}"

test_pass "issue-2280: --color flag with Helm 4"
