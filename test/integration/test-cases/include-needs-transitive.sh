#!/usr/bin/env bash

# Test case for --include-needs vs --include-transitive-needs
# This test verifies that:
# 1. --include-needs includes only direct dependencies
# 2. --include-transitive-needs includes all transitive dependencies
# 3. Log message shows correct count of releases matching selector (not including needs)

include_needs_case_input_dir="${cases_dir}/include-needs-transitive/input"
include_needs_tmp=$(mktemp -d)

test_start "include-needs vs include-transitive-needs"

# Test 1: --include-needs should include only direct dependencies
info "Testing --include-needs includes only direct dependencies"
${helmfile} -f ${include_needs_case_input_dir}/helmfile.yaml -l name=service-a template --include-needs > ${include_needs_tmp}/include-needs.log 2>&1 || fail "helmfile template --include-needs should not fail"

# Verify that service-a, service-b are included in the output (service-b is direct need of service-a)
# service-c should NOT be included (it's transitive, not direct)
include_needs_output=$(cat ${include_needs_tmp}/include-needs.log)

if echo "${include_needs_output}" | grep -q "name: service-a" && \
   echo "${include_needs_output}" | grep -q "name: service-b" && \
   ! echo "${include_needs_output}" | grep -q "name: service-c"; then
    info "--include-needs correctly includes only direct dependencies (service-a, service-b)"
else
    cat ${include_needs_tmp}/include-needs.log
    fail "--include-needs should include only service-a and service-b (direct need), not service-c (transitive)"
fi

# Verify log shows "1 release(s) matching name=service-a" (not 2 or 3)
if echo "${include_needs_output}" | grep -q "1 release(s) matching name=service-a"; then
    info "Log correctly shows 1 release matching selector"
else
    cat ${include_needs_tmp}/include-needs.log
    fail "Log should show '1 release(s) matching name=service-a', not including needs count"
fi

# Test 2: --include-transitive-needs should include all transitive dependencies
info "Testing --include-transitive-needs includes all transitive dependencies"
${helmfile} -f ${include_needs_case_input_dir}/helmfile.yaml -l name=service-a template --include-transitive-needs > ${include_needs_tmp}/include-transitive-needs.log 2>&1 || fail "helmfile template --include-transitive-needs should not fail"

# Verify that service-a, service-b, service-c are all included
transitive_output=$(cat ${include_needs_tmp}/include-transitive-needs.log)

if echo "${transitive_output}" | grep -q "name: service-a" && \
   echo "${transitive_output}" | grep -q "name: service-b" && \
   echo "${transitive_output}" | grep -q "name: service-c"; then
    info "--include-transitive-needs correctly includes all transitive dependencies (service-a, service-b, service-c)"
else
    cat ${include_needs_tmp}/include-transitive-needs.log
    fail "--include-transitive-needs should include service-a, service-b, and service-c (transitive)"
fi

# Verify log still shows "1 release(s) matching name=service-a" (selector match, not total)
if echo "${transitive_output}" | grep -q "1 release(s) matching name=service-a"; then
    info "Log correctly shows 1 release matching selector (not including transitive needs)"
else
    cat ${include_needs_tmp}/include-transitive-needs.log
    fail "Log should show '1 release(s) matching name=service-a', not including needs count"
fi

# Test 3: Verify service-d is never included (not in dependency chain)
if ! echo "${include_needs_output}" | grep -q "name: service-d" && \
   ! echo "${transitive_output}" | grep -q "name: service-d"; then
    info "service-d correctly not included (not in dependency chain)"
else
    fail "service-d should never be included as it's not in the dependency chain"
fi

# Cleanup
rm -rf ${include_needs_tmp}

test_pass "include-needs vs include-transitive-needs"
