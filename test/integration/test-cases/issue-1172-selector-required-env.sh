#!/usr/bin/env bash

# Test for issue #1172: Helmfile renders entire helmfile even with selector labels
# https://github.com/helmfile/helmfile/issues/1172
#
# This test verifies that selector-based filtering works correctly when
# requiredEnv is used in release values. Users set all required env vars,
# so rendering succeeds, and selectors filter out non-matching releases.

issue_1172_case_dir="$(cd "${cases_dir}/issue-1172-selector-required-env" && pwd)"
issue_1172_tmp=$(mktemp -d)

cleanup_issue_1172() {
    rm -rf "${issue_1172_tmp}"
}
trap cleanup_issue_1172 EXIT

test_start "issue-1172: selector filtering with requiredEnv"

export ISSUE_1172_BUZZ="test-buzz-value"

# Test 1: With selector tier=label2, only rel2 should be templated
info "Test 1: helmfile template with -l tier=label2 should only template rel2"
code=0
${helmfile} -f "${issue_1172_case_dir}/input/helmfile.yaml.gotmpl" \
    template -l tier=label2 > "${issue_1172_tmp}/selected.yaml" 2>&1 || code=$?

if [ ${code} -ne 0 ]; then
    cat "${issue_1172_tmp}/selected.yaml"
    fail "helmfile template with selector should succeed when all env vars are set"
fi

if grep -q "release: rel1" "${issue_1172_tmp}/selected.yaml"; then
    fail "rel1 should NOT be templated when selector tier=label2 is active"
fi

if ! grep -q "release: rel2" "${issue_1172_tmp}/selected.yaml"; then
    cat "${issue_1172_tmp}/selected.yaml"
    fail "rel2 should be templated when selector tier=label2 is active"
fi

info "PASS: only rel2 was templated with selector"

# Test 2: Without selector, both releases should be templated
info "Test 2: helmfile template without selector should template both releases"
code=0
${helmfile} -f "${issue_1172_case_dir}/input/helmfile.yaml.gotmpl" \
    template > "${issue_1172_tmp}/all.yaml" 2>&1 || code=$?

if [ ${code} -ne 0 ]; then
    cat "${issue_1172_tmp}/all.yaml"
    fail "helmfile template without selector should succeed when all env vars are set"
fi

if ! grep -q "release: rel1" "${issue_1172_tmp}/all.yaml"; then
    fail "rel1 should be templated when no selector is active"
fi

if ! grep -q "release: rel2" "${issue_1172_tmp}/all.yaml"; then
    fail "rel2 should be templated when no selector is active"
fi

info "PASS: both releases were templated without selector"

trap - EXIT
cleanup_issue_1172
test_pass "issue-1172: selector filtering with requiredEnv"
