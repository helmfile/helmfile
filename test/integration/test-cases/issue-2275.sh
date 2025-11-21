#!/usr/bin/env bash

# Test for issue #2275: helmfile should detect cluster version and pass to helm-diff
# Without this fix, helm-diff falls back to v1.20.0 and fails for charts requiring newer versions

issue_2275_input_dir="${cases_dir}/issue-2275/input"
issue_2275_tmp_dir=$(mktemp -d)

cd "${issue_2275_input_dir}"

test_start "issue-2275: auto-detect kubernetes version for helm-diff"

info "Testing helmfile apply with chart requiring Kubernetes >=1.25.0"
info "Expected: Success with auto-detected cluster version"

# Test 1: Apply should succeed with auto-detected cluster version
${helmfile} apply --skip-diff-on-install --suppress-diff > "${issue_2275_tmp_dir}/test-2275-output.txt" 2>&1
code=$?

if [ $code -ne 0 ]; then
  if grep -q "incompatible with Kubernetes v1.20.0" "${issue_2275_tmp_dir}/test-2275-output.txt"; then
    cat "${issue_2275_tmp_dir}/test-2275-output.txt"
    rm -rf "${issue_2275_tmp_dir}"
    fail "Issue #2275 regression: helm-diff fell back to v1.20.0"
  else
    cat "${issue_2275_tmp_dir}/test-2275-output.txt"
    rm -rf "${issue_2275_tmp_dir}"
    fail "Unexpected error during apply"
  fi
fi

info "Chart installed successfully with auto-detected version"

# Test 2: Diff should work with auto-detected version
info "Testing helmfile diff with auto-detected cluster version"

${helmfile} diff > "${issue_2275_tmp_dir}/test-2275-diff-output.txt" 2>&1
code=$?

if [ $code -ne 0 ] && [ $code -ne 2 ]; then
  if grep -q "incompatible with Kubernetes v1.20.0" "${issue_2275_tmp_dir}/test-2275-diff-output.txt"; then
    cat "${issue_2275_tmp_dir}/test-2275-diff-output.txt"
    rm -rf "${issue_2275_tmp_dir}"
    fail "Issue #2275 regression in diff: helm-diff fell back to v1.20.0"
  else
    cat "${issue_2275_tmp_dir}/test-2275-diff-output.txt"
    rm -rf "${issue_2275_tmp_dir}"
    fail "Unexpected error during diff"
  fi
fi

info "Diff succeeded with auto-detected version"

# Test 3: Second apply (upgrade scenario) - this is the critical test case from issue #2275
# The first apply worked with --skip-diff-on-install, but second apply would fail without the fix
info "Testing second helmfile apply (upgrade scenario) - critical test for issue #2275"
info "Modifying chart to trigger an actual upgrade..."

# Update chart version to trigger an upgrade
sed -i.bak 's/version: 1.0.0/version: 1.0.1/' test-chart/Chart.yaml

info "Running helmfile apply to upgrade chart (this will run diff)"
info "This would fail with 'incompatible with Kubernetes v1.20.0' before the fix"

${helmfile} apply --suppress-diff > "${issue_2275_tmp_dir}/test-2275-apply2-output.txt" 2>&1
code=$?

# Restore original chart version
mv test-chart/Chart.yaml.bak test-chart/Chart.yaml

if [ $code -ne 0 ]; then
  if grep -q "incompatible with Kubernetes v1.20.0" "${issue_2275_tmp_dir}/test-2275-apply2-output.txt"; then
    cat "${issue_2275_tmp_dir}/test-2275-apply2-output.txt"
    rm -rf "${issue_2275_tmp_dir}"
    fail "Issue #2275 regression: upgrade failed - helm-diff fell back to v1.20.0"
  else
    cat "${issue_2275_tmp_dir}/test-2275-apply2-output.txt"
    rm -rf "${issue_2275_tmp_dir}"
    fail "Unexpected error during upgrade"
  fi
fi

info "Upgrade succeeded with auto-detected version"

# Cleanup
${helm} uninstall test-release-2275 --namespace default 2>/dev/null || true
rm -rf "${issue_2275_tmp_dir}"

test_pass "issue-2275: auto-detect kubernetes version for helm-diff"
