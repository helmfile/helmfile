#!/usr/bin/env bash

# Test for issue #2431: Local chart with external dependencies
# 
# helmfile.yaml has repos configured (vector), but NOT the repo that the
# local chart depends on (wiremind). The fix ensures that:
# 1. helm dep build does NOT receive --skip-refresh for local charts
# 2. helmfile template succeeds without "no cached repository" error
#
# This replicates the exact scenario from issue #2431.

issue_2431_input_dir="${cases_dir}/issue-2431/input"
issue_2431_tmp=$(mktemp -d)
issue_2431_output="${issue_2431_tmp}/template.log"

cleanup_issue_2431() {
  rm -rf "${issue_2431_tmp}"
}
trap cleanup_issue_2431 EXIT

test_start "issue-2431: Local chart with external dependency not in helmfile.yaml"

info "Running helmfile template: local chart depends on wiremind repo (not configured in helmfile.yaml)"
${helmfile} -f "${issue_2431_input_dir}/helmfile.yaml" -e karma template > "${issue_2431_output}" 2>&1 || {
  code=$?
  cat "${issue_2431_output}"
  
  # Check if the failure is due to "no cached repository" or "no repository definition" error
  if grep -q "no cached repository" "${issue_2431_output}" || grep -q "no repository definition" "${issue_2431_output}"; then
    fail "Issue #2431 regression: helm dep build received --skip-refresh incorrectly for local chart"
  fi
  
  fail "helmfile template failed with exit code ${code}"
}

info "SUCCESS: helmfile template completed successfully"
info "Template output:"
cat "${issue_2431_output}"

trap - EXIT
test_pass "issue-2431: Local chart with external dependency not in helmfile.yaml"
