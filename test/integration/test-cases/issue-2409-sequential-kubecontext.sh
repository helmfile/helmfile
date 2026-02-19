# Issue #2409: Test that --sequential-helmfiles works with kubeContext from environment values
# https://github.com/helmfile/helmfile/issues/2409
#
# This test verifies that helmfile --sequential-helmfiles correctly resolves
# paths (including bases and environment values) without changing the process
# working directory. Previously, the sequential code path used os.Chdir(),
# which broke relative path resolution for KUBECONFIG and similar variables.

issue_2409_input_dir="${cases_dir}/issue-2409-sequential-kubecontext/input"
issue_2409_tmp=$(mktemp -d)

# Test 1: sequential mode with environment-defined kubeContext
test_start "issue 2409 sequential helmfiles with kubeContext"

info "Running helmfile --sequential-helmfiles template with kubeContext from environment"
${helmfile} --sequential-helmfiles -f ${issue_2409_input_dir} template -e test \
    > ${issue_2409_tmp}/sequential.log 2>&1 \
    || fail "\"helmfile --sequential-helmfiles template -e test\" shouldn't fail"

# Test 2: verify output matches parallel mode
info "Running helmfile template in parallel mode for comparison"
${helmfile} -f ${issue_2409_input_dir} template -e test \
    > ${issue_2409_tmp}/parallel.log 2>&1 \
    || fail "\"helmfile template -e test\" shouldn't fail"

diff -u ${issue_2409_tmp}/parallel.log ${issue_2409_tmp}/sequential.log \
    || fail "sequential and parallel output should match"

rm -rf ${issue_2409_tmp}

test_pass "issue 2409 sequential helmfiles with kubeContext"
