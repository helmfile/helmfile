#!/usr/bin/env bash

# Test for issue #2431: Local chart with external dependencies
# 
# When helmfile.yaml has non-OCI repos configured but the local chart has
# dependencies on repos NOT in helmfile.yaml, the fix ensures that:
# 1. helm dep build does NOT receive --skip-refresh for local charts
# 2. helmfile template succeeds without "no cached repository" error
#
# This test uses a simple local chart without external dependencies to verify
# the basic flow works correctly. The core logic is tested in unit tests.

issue_2431_input_dir="${cases_dir}/issue-2431/input"
issue_2431_tmp=$(mktemp -d)
issue_2431_output="${issue_2431_tmp}/template.log"

cleanup_issue_2431() {
  rm -rf "${issue_2431_tmp}"
}
trap cleanup_issue_2431 EXIT

test_start "issue-2431: Local chart with repos configured in helmfile.yaml"

info "Running helmfile template with non-OCI repos + local chart"
${helmfile} -f "${issue_2431_input_dir}/helmfile.yaml" template > "${issue_2431_output}" 2>&1 || {
  code=$?
  cat "${issue_2431_output}"
  
  # Check if the failure is due to "no cached repository" or "no repository definition" error
  if grep -q "no cached repository" "${issue_2431_output}" || grep -q "no repository definition" "${issue_2431_output}"; then
    fail "Issue #2431 regression: helm dep build received --skip-refresh incorrectly"
  fi
  
  fail "helmfile template failed with exit code ${code}"
}

info "SUCCESS: helmfile template completed successfully"
info "Template output:"
cat "${issue_2431_output}"

trap - EXIT
test_pass "issue-2431: Local chart with repos configured in helmfile.yaml"
