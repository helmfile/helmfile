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
${helmfile} -f ${include_needs_case_input_dir}/helmfile.yaml -l name=serviceA template --include-needs > ${include_needs_tmp}/include-needs.log 2>&1 || fail "helmfile template --include-needs should not fail"

# Verify that serviceA, serviceB are included in the output (serviceB is direct need of serviceA)
# serviceC should NOT be included (it's transitive, not direct)
include_needs_output=$(cat ${include_needs_tmp}/include-needs.log)

if echo "${include_needs_output}" | grep -q "name: serviceA" && \
   echo "${include_needs_output}" | grep -q "name: serviceB" && \
   ! echo "${include_needs_output}" | grep -q "name: serviceC"; then
    info "--include-needs correctly includes only direct dependencies (serviceA, serviceB)"
else
    cat ${include_needs_tmp}/include-needs.log
    fail "--include-needs should include only serviceA and serviceB (direct need), not serviceC (transitive)"
fi

# Verify log shows "1 release(s) matching name=serviceA" (not 2 or 3)
if echo "${include_needs_output}" | grep -q "1 release(s) matching name=serviceA"; then
    info "Log correctly shows 1 release matching selector"
else
    cat ${include_needs_tmp}/include-needs.log
    fail "Log should show '1 release(s) matching name=serviceA', not including needs count"
fi

# Test 2: --include-transitive-needs should include all transitive dependencies
info "Testing --include-transitive-needs includes all transitive dependencies"
${helmfile} -f ${include_needs_case_input_dir}/helmfile.yaml -l name=serviceA template --include-transitive-needs > ${include_needs_tmp}/include-transitive-needs.log 2>&1 || fail "helmfile template --include-transitive-needs should not fail"

# Verify that serviceA, serviceB, serviceC are all included
transitive_output=$(cat ${include_needs_tmp}/include-transitive-needs.log)

if echo "${transitive_output}" | grep -q "name: serviceA" && \
   echo "${transitive_output}" | grep -q "name: serviceB" && \
   echo "${transitive_output}" | grep -q "name: serviceC"; then
    info "--include-transitive-needs correctly includes all transitive dependencies (serviceA, serviceB, serviceC)"
else
    cat ${include_needs_tmp}/include-transitive-needs.log
    fail "--include-transitive-needs should include serviceA, serviceB, and serviceC (transitive)"
fi

# Verify log still shows "1 release(s) matching name=serviceA" (selector match, not total)
if echo "${transitive_output}" | grep -q "1 release(s) matching name=serviceA"; then
    info "Log correctly shows 1 release matching selector (not including transitive needs)"
else
    cat ${include_needs_tmp}/include-transitive-needs.log
    fail "Log should show '1 release(s) matching name=serviceA', not including needs count"
fi

# Test 3: Verify serviceD is never included (not in dependency chain)
if ! echo "${include_needs_output}" | grep -q "name: serviceD" && \
   ! echo "${transitive_output}" | grep -q "name: serviceD"; then
    info "serviceD correctly not included (not in dependency chain)"
else
    fail "serviceD should never be included as it's not in the dependency chain"
fi

# Cleanup
rm -rf ${include_needs_tmp}

test_pass "include-needs vs include-transitive-needs"
