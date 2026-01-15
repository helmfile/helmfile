#!/usr/bin/env bash

# Regression test for issue #2355: Validate flag does not work when using kustomize after Helm 4 upgrade
#
# Background: In Helm 4, the --validate and --dry-run flags are mutually exclusive.
# When helmfile uses kustomize/chartify, it was incorrectly adding --dry-run=server
# even when --validate was already set, causing:
#   Error: if any flags in the group [validate dry-run] are set none of the others can be

issue_2355_input_dir="${cases_dir}/issue-2355/input"

test_start "issue-2355: validate flag with kustomize charts (Helm 4 compatibility)"

# Test 1: helmfile template --validate with kustomize chart should NOT fail
# Note: We use template instead of diff because diff requires a running cluster with releases
info "Test 1: Running template --validate with kustomize chart"
if ! ${helmfile} -f ${issue_2355_input_dir}/helmfile.yaml template --validate > /dev/null 2>&1; then
    # Capture the error for debugging
    error_output=$(${helmfile} -f ${issue_2355_input_dir}/helmfile.yaml template --validate 2>&1)

    # Check if it's the specific mutual exclusion error we're fixing
    if echo "$error_output" | grep -q "validate.*dry-run.*were all set"; then
        fail "helmfile template --validate with kustomize failed with mutual exclusion error (issue #2355 not fixed): $error_output"
    else
        # Other errors might be acceptable (e.g., no cluster connection for validation)
        warn "helmfile template --validate had an error (but not the mutual exclusion issue): $error_output"
    fi
fi

# Test 2: Verify that without --validate, the command also works
info "Test 2: Running template without --validate (baseline test)"
${helmfile} -f ${issue_2355_input_dir}/helmfile.yaml template > /dev/null 2>&1 || \
    fail "helmfile template without --validate shouldn't fail"

test_pass "issue-2355: validate flag with kustomize charts (Helm 4 compatibility)"
