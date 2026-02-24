#!/usr/bin/env bash

# Test for issue #2418: Skip helm repo update when only OCI repos are configured
# When using only OCI repositories combined with a local chart, helmfile should NOT
# attempt to run `helm repo update` which would fail with "no repositories found".

issue_2418_input_dir="${cases_dir}/issue-2418/input"
issue_2418_tmp=$(mktemp -d)
issue_2418_output="${issue_2418_tmp}/template.log"

cleanup_issue_2418() {
  rm -rf "${issue_2418_tmp}"
}
trap cleanup_issue_2418 EXIT

test_start "issue-2418: OCI repos + local chart should skip helm repo update"

# Run helmfile template - this would fail with "no repositories found" before the fix
# because it would attempt to run `helm repo update` for OCI repos
info "Running helmfile template with OCI repo + local chart"
${helmfile} -f "${issue_2418_input_dir}/helmfile.yaml" template > "${issue_2418_output}" 2>&1 || {
  code=$?
  cat "${issue_2418_output}"
  
  # Check if the failure is due to "no repositories found" error
  if grep -q "no repositories found" "${issue_2418_output}"; then
    fail "Issue #2418 regression: helm repo update was called for OCI-only repos"
  fi
  
  fail "helmfile template failed with exit code ${code}"
}

info "SUCCESS: helmfile template completed without 'no repositories found' error"
info "Template output:"
cat "${issue_2418_output}"

trap - EXIT
test_pass "issue-2418: OCI repos + local chart should skip helm repo update"
