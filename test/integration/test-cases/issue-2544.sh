#!/usr/bin/env bash

# Test for issue #2544: subhelmfiles should only be evaluated if their selectors match
# When CLI selectors are provided, subhelmfiles with incompatible selectors are skipped.

issue_2544_input_dir="${cases_dir}/issue-2544/input"
issue_2544_tmp_dir=$(mktemp -d)

cleanup_issue_2544() {
    rm -rf "${issue_2544_tmp_dir}"
}
trap cleanup_issue_2544 EXIT

test_start "issue-2544: subhelmfiles skipped when selectors conflict with CLI"

# --- Test 1: -l app=b should only produce output from subhelmfile-b ---------------------------

info "Running helmfile template -l app=b"
${helmfile} -f "${issue_2544_input_dir}/helmfile.yaml" template -l app=b \
    > "${issue_2544_tmp_dir}/output-b.txt" 2>&1 || {
    cat "${issue_2544_tmp_dir}/output-b.txt"
    fail "helmfile template -l app=b failed"
}

# Should contain release-b
grep -q "release-b" "${issue_2544_tmp_dir}/output-b.txt" \
    || fail "output should contain release-b (matching selector app=b)"

# Should NOT contain release-a or release-c (their subhelmfiles have incompatible selectors)
grep -q "release-a" "${issue_2544_tmp_dir}/output-b.txt" \
    && fail "output should NOT contain release-a (subhelmfile selectors app=a conflict with app=b)"

grep -q "release-c" "${issue_2544_tmp_dir}/output-b.txt" \
    && fail "output should NOT contain release-c (subhelmfile selectors app=c conflict with app=b)"

info "PASS: -l app=b produces only release-b"

# --- Test 2: no selector should produce all releases -----------------------------------------

info "Running helmfile template without selector"
${helmfile} -f "${issue_2544_input_dir}/helmfile.yaml" template \
    > "${issue_2544_tmp_dir}/output-all.txt" 2>&1 || {
    cat "${issue_2544_tmp_dir}/output-all.txt"
    fail "helmfile template without selector failed"
}

grep -q "release-a" "${issue_2544_tmp_dir}/output-all.txt" \
    || fail "output should contain release-a"
grep -q "release-b" "${issue_2544_tmp_dir}/output-all.txt" \
    || fail "output should contain release-b"
grep -q "release-c" "${issue_2544_tmp_dir}/output-all.txt" \
    || fail "output should contain release-c"

info "PASS: no selector produces all releases"

# --- Test 3: -l app=a should skip subhelmfiles with selectors app=b and app=c ---------------

info "Running helmfile template -l app=a"
${helmfile} -f "${issue_2544_input_dir}/helmfile.yaml" template -l app=a \
    > "${issue_2544_tmp_dir}/output-a.txt" 2>&1 || {
    cat "${issue_2544_tmp_dir}/output-a.txt"
    fail "helmfile template -l app=a failed"
}

grep -q "release-a" "${issue_2544_tmp_dir}/output-a.txt" \
    || fail "output should contain release-a (matching selector app=a)"

grep -q "release-b" "${issue_2544_tmp_dir}/output-a.txt" \
    && fail "output should NOT contain release-b (subhelmfile selectors app=b conflict with app=a)"

grep -q "release-c" "${issue_2544_tmp_dir}/output-a.txt" \
    && fail "output should NOT contain release-c (subhelmfile selectors app=c conflict with app=a)"

info "PASS: -l app=a produces only release-a"

trap - EXIT
test_pass "issue-2544: subhelmfiles skipped when selectors conflict with CLI"
