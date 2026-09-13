# Issue #2712: Support Helm 4 --rollback-on-failure alongside deprecated --atomic
# https://github.com/helmfile/helmfile/issues/2712
#
# End-to-end coverage:
#   1. atomic: true is accepted (no unknown-field parse error), deploys, and emits the
#      version-correct flag: --rollback-on-failure on Helm 4 (auto-migration of the
#      deprecated --atomic), --atomic on Helm 3.
#   2. rollbackOnFailure: true emits --rollback-on-failure on Helm 4 and is rejected
#      with a clear Helm-4-required error on Helm 3.
#
# The flag-emission assertions inspect the `exec: helm upgrade --install ...` lines
# that helmfile logs under --debug (see pkg/helmexec/exec.go exec helper).

issue_2712_input_dir="${cases_dir}/issue-2712-rollback-on-failure/input"
issue_2712_tmp=$(mktemp -d)

cleanup_issue_2712() {
    rm -rf "${issue_2712_tmp}"
}
trap cleanup_issue_2712 EXIT

# Succeeds when the captured --debug log contains a `helm upgrade --install` command
# carrying the given flag. The flag is matched as a standalone argument (surrounded by
# whitespace) so a release name such as "issue-2712-atomic" can never be confused with
# the "--atomic" flag. $1 = log file, $2 = flag token (e.g. --rollback-on-failure).
issue_2712_assert_upgrade_flag() {
    grep -E "upgrade --install" "$1" | grep -qE "(^|[[:space:]])${2}([[:space:]]|$)"
}

# --- Test 1: atomic: true is accepted, deploys, and emits the version-correct flag ---
if [ "${HELMFILE_HELM4}" = "1" ]; then
    test_start "issue-2712 atomic migrates to --rollback-on-failure (Helm 4)"
else
    test_start "issue-2712 atomic emits --atomic (Helm 3)"
fi

info "Syncing release with atomic: true"
if ! ${helmfile} --debug -f "${issue_2712_input_dir}/atomic.yaml" sync > "${issue_2712_tmp}/atomic.log" 2>&1; then
    cat "${issue_2712_tmp}/atomic.log"
    fail "helmfile sync with atomic: true should succeed"
fi

info "Verifying ConfigMap issue-2712-atomic-cm was applied"
${kubectl} get configmap issue-2712-atomic-cm > /dev/null \
    || fail "ConfigMap issue-2712-atomic-cm should exist after sync"

if [ "${HELMFILE_HELM4}" = "1" ]; then
    issue_2712_assert_upgrade_flag "${issue_2712_tmp}/atomic.log" "--rollback-on-failure" \
        || { cat "${issue_2712_tmp}/atomic.log"; fail "Helm 4 should migrate atomic:true to --rollback-on-failure"; }
    if issue_2712_assert_upgrade_flag "${issue_2712_tmp}/atomic.log" "--atomic"; then
        fail "Helm 4 should not emit the deprecated --atomic flag for atomic:true"
    fi
    test_pass "issue-2712 atomic migrates to --rollback-on-failure (Helm 4)"
else
    issue_2712_assert_upgrade_flag "${issue_2712_tmp}/atomic.log" "--atomic" \
        || { cat "${issue_2712_tmp}/atomic.log"; fail "Helm 3 should emit --atomic for atomic:true"; }
    test_pass "issue-2712 atomic emits --atomic (Helm 3)"
fi

# --- Test 2: rollbackOnFailure behavior is version-gated ---
if [ "${HELMFILE_HELM4}" = "1" ]; then
    test_start "issue-2712 rollbackOnFailure emits --rollback-on-failure (Helm 4)"
    info "Syncing release with rollbackOnFailure: true"
    if ! ${helmfile} --debug -f "${issue_2712_input_dir}/rollback.yaml" sync > "${issue_2712_tmp}/rollback.log" 2>&1; then
        cat "${issue_2712_tmp}/rollback.log"
        fail "helmfile sync with rollbackOnFailure: true should succeed on Helm 4"
    fi
    ${kubectl} get configmap issue-2712-rollback-cm > /dev/null \
        || fail "ConfigMap issue-2712-rollback-cm should exist after sync"
    issue_2712_assert_upgrade_flag "${issue_2712_tmp}/rollback.log" "--rollback-on-failure" \
        || { cat "${issue_2712_tmp}/rollback.log"; fail "rollbackOnFailure:true should emit --rollback-on-failure on Helm 4"; }
    test_pass "issue-2712 rollbackOnFailure emits --rollback-on-failure (Helm 4)"
else
    test_start "issue-2712 rollbackOnFailure rejected on Helm 3"
    info "Syncing release with rollbackOnFailure: true (expected to fail)"
    if ${helmfile} -f "${issue_2712_input_dir}/rollback.yaml" sync > "${issue_2712_tmp}/rollback.log" 2>&1; then
        cat "${issue_2712_tmp}/rollback.log"
        fail "helmfile sync with rollbackOnFailure: true should fail on Helm 3"
    fi
    grep -q "rollbackOnFailure requires Helm 4" "${issue_2712_tmp}/rollback.log" \
        || { cat "${issue_2712_tmp}/rollback.log"; fail "expected 'rollbackOnFailure requires Helm 4' error on Helm 3"; }
    test_pass "issue-2712 rollbackOnFailure rejected on Helm 3"
fi

# --- Cleanup ---
info "Cleaning up issue-2712 releases"
${helmfile} -f "${issue_2712_input_dir}/atomic.yaml" destroy > /dev/null 2>&1 || true
${helmfile} -f "${issue_2712_input_dir}/rollback.yaml" destroy > /dev/null 2>&1 || true
