# Issue #2409: Test that --sequential-helmfiles works with multiple helmfile.d files
# https://github.com/helmfile/helmfile/issues/2409
#
# This test verifies that helmfile --sequential-helmfiles correctly processes
# multiple files without changing the process working directory. The fix uses
# baseDir for path resolution instead of os.Chdir(), which previously broke
# relative path resolution for KUBECONFIG and similar environment variables.

issue_2409_input_dir="${cases_dir}/issue-2409-sequential-kubecontext/input/helmfile.d"
issue_2409_tmp=$(mktemp -d)

test_start "issue 2409 sequential helmfiles with kubeContext"

info "Running helmfile --sequential-helmfiles template with multiple files"
${helmfile} --sequential-helmfiles -f ${issue_2409_input_dir} template \
    > ${issue_2409_tmp}/sequential.log 2>&1 \
    || fail "\"helmfile --sequential-helmfiles template\" shouldn't fail"

# Verify both releases from separate files are present in output
grep -q "test-release-2409" ${issue_2409_tmp}/sequential.log \
    || fail "release test-release-2409 from 01-app.yaml should be in output"

grep -q "test-infra-2409" ${issue_2409_tmp}/sequential.log \
    || fail "release test-infra-2409 from 02-infra.yaml should be in output"

# Verify kubeContext was correctly applied
grep -q "test-context" ${issue_2409_tmp}/sequential.log \
    || fail "kubeContext 'test-context' should appear in template output"

rm -rf ${issue_2409_tmp}

test_pass "issue 2409 sequential helmfiles with kubeContext"
