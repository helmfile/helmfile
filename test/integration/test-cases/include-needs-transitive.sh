#!/usr/bin/env bash

# Test case for issue #1003: --include-needs should only include direct dependencies
# while --include-transitive-needs should include both direct and transitive dependencies

test_case_dir="${cases_dir}/include-needs-transitive"
input_dir="${test_case_dir}/input"

tmp_dir=$(mktemp -d)
trap 'rm -rf "$tmp_dir"' EXIT

test_start "include-needs vs include-transitive-needs"

info "https://github.com/helmfile/helmfile/issues/1003"

# Test 1: --include-needs should only include direct dependencies
info "Test 1: template with --include-needs should only include direct dependencies"
template_output="${tmp_dir}/include-needs.log"
"${helmfile}" -f "${input_dir}/helmfile.yaml" -l name=release3 template --include-needs > "${template_output}" 2>&1
code=$?
if [ ${code} -ne 0 ]; then
    cat "${template_output}"
    fail "helmfile template with --include-needs should not fail"
fi

if grep -q "release1" "${template_output}"; then
    fail "--include-needs should NOT include transitive dependency release1"
fi

if ! grep -q "release2" "${template_output}"; then
    fail "--include-needs should include direct dependency release2"
fi

if ! grep -q "release3" "${template_output}"; then
    fail "--include-needs should include selected release release3"
fi

info "PASS: --include-needs only includes direct dependencies"

# Test 2: --include-transitive-needs should include all dependencies
info "Test 2: template with --include-transitive-needs should include all dependencies"
template_output="${tmp_dir}/include-transitive-needs.log"
"${helmfile}" -f "${input_dir}/helmfile.yaml" -l name=release3 template --include-transitive-needs > "${template_output}" 2>&1
code=$?
if [ ${code} -ne 0 ]; then
    cat "${template_output}"
    fail "helmfile template with --include-transitive-needs should not fail"
fi

if ! grep -q "release1" "${template_output}"; then
    fail "--include-transitive-needs should include transitive dependency release1"
fi

if ! grep -q "release2" "${template_output}"; then
    fail "--include-transitive-needs should include direct dependency release2"
fi

if ! grep -q "release3" "${template_output}"; then
    fail "--include-transitive-needs should include selected release release3"
fi

info "PASS: --include-transitive-needs includes all dependencies"

# Test 3: Verify same behavior for lint command
info "Test 3: lint with --include-needs should only include direct dependencies"
lint_output="${tmp_dir}/lint-include-needs.log"
"${helmfile}" -f "${input_dir}/helmfile.yaml" -l name=release3 lint --include-needs > "${lint_output}" 2>&1
code=$?
if [ ${code} -ne 0 ]; then
    cat "${lint_output}"
    fail "helmfile lint with --include-needs should not fail"
fi

if grep -q "release1" "${lint_output}"; then
    fail "lint with --include-needs should NOT include transitive dependency release1"
fi

if ! grep -q "release2" "${lint_output}"; then
    fail "lint with --include-needs should include direct dependency release2"
fi

if ! grep -q "release3" "${lint_output}"; then
    fail "lint with --include-needs should include selected release release3"
fi

info "PASS: lint with --include-needs only includes direct dependencies"

# Test 4: Verify same behavior for diff command  
info "Test 4: diff with --include-needs should only include direct dependencies"
diff_output="${tmp_dir}/diff-include-needs.log"
"${helmfile}" -f "${input_dir}/helmfile.yaml" -l name=release3 diff --include-needs > "${diff_output}" 2>&1
code=$?
if [ ${code} -ne 0 ]; then
    cat "${diff_output}"
    fail "helmfile diff with --include-needs should not fail"
fi

if grep -q "release1" "${diff_output}"; then
    fail "diff with --include-needs should NOT include transitive dependency release1"
fi

if ! grep -q "release2" "${diff_output}"; then
    fail "diff with --include-needs should include direct dependency release2"
fi

if ! grep -q "release3" "${diff_output}"; then
    fail "diff with --include-needs should include selected release release3"
fi

info "PASS: diff with --include-needs only includes direct dependencies"

test_pass "include-needs vs include-transitive-needs"
