#!/usr/bin/env bash

# Regression test for issue #2355: Validate flag does not work when using kustomize after Helm 4 upgrade
#
# Background: In Helm 4, the --validate and --dry-run flags are mutually exclusive.
# When helmfile uses kustomize/chartify, it was incorrectly adding --dry-run=server
# even when --validate was already set, causing:
#   Error: if any flags in the group [validate dry-run] are set none of the others can be

issue_2355_input_dir="${cases_dir}/issue-2355/input"

test_start "issue-2355: validate flag with kustomize charts (Helm 4 compatibility)"

# Test 1: helmfile diff --validate with kustomize chart should NOT fail due to validate/dry-run mutual exclusion
# We deliberately use diff here because it is a cluster-requiring command that can trigger --dry-run=server via chartify.
info "Test 1: Running diff --validate with kustomize chart"
error_output=$(${helmfile} -f ${issue_2355_input_dir}/helmfile.yaml diff --validate 2>&1)
exit_code=$?
if [ $exit_code -ne 0 ]; then
    # Check if it's the specific mutual exclusion error we're fixing
    if echo "$error_output" | grep -q "validate.*dry-run.*were all set"; then
        fail "helmfile diff --validate with kustomize failed with mutual exclusion error (issue #2355 not fixed): $error_output"
    else
        # Other errors might be acceptable (e.g., no cluster connection for validation, or non-zero diff exit codes)
        warn "helmfile diff --validate had an error (but not the mutual exclusion issue): $error_output"
    fi
fi

# Test 2: Verify that without --validate, the command does not hit the mutual exclusion error
info "Test 2: Running diff without --validate (baseline test for flag interaction)"
error_output=$(${helmfile} -f ${issue_2355_input_dir}/helmfile.yaml diff 2>&1)
exit_code=$?
if [ $exit_code -ne 0 ]; then
    if echo "$error_output" | grep -q "validate.*dry-run.*were all set"; then
        fail "helmfile diff without --validate failed with mutual exclusion error (issue #2355 not fixed): $error_output"
    else
        # Non-zero exit codes from diff (e.g., detected differences) or other errors are tolerated here.
        warn "helmfile diff without --validate had an error (but not the mutual exclusion issue): $error_output"
    fi
fi

test_pass "issue-2355: validate flag with kustomize charts (Helm 4 compatibility)"
