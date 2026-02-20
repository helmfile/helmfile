# Issue #2409: Test that --sequential-helmfiles works with kubeContext from bases
# https://github.com/helmfile/helmfile/issues/2409
#
# This test replicates the exact user scenario:
#   - Multiple files in helmfile.d/
#   - bases: with relative paths (../bases/) for environments and defaults
#   - Environment values set kubeContext (resolved via .Environment.Values.kubeContext)
#   - Running diff against the minikube cluster to exercise kubeContext resolution
#
# Before the fix, os.Chdir() in the sequential path could break relative path
# resolution, causing kubeContext or KUBECONFIG to not resolve correctly.

issue_2409_input_dir="${cases_dir}/issue-2409-sequential-kubecontext/input/helmfile.d"
issue_2409_tmp=$(mktemp -d)

test_start "issue 2409 sequential helmfiles with kubeContext"

# Run diff with --sequential-helmfiles to verify kubeContext resolves correctly
# against the minikube cluster. This would fail with "context does not exist"
# if path resolution were broken by os.Chdir().
info "Running helmfile --sequential-helmfiles diff with kubeContext from bases"
bash -c "${helmfile} --sequential-helmfiles -f ${issue_2409_input_dir} diff --detailed-exitcode; code="'$?'"; [ "'${code}'" -eq 2 ]" \
    || fail "\"helmfile --sequential-helmfiles diff\" should exit 2 (changes detected)"

# Also verify template output contains both releases and the relative values file
info "Running helmfile --sequential-helmfiles template"
${helmfile} --sequential-helmfiles -f ${issue_2409_input_dir} template \
    > ${issue_2409_tmp}/sequential.log 2>&1 \
    || fail "\"helmfile --sequential-helmfiles template\" shouldn't fail"

grep -q "test-release-2409" ${issue_2409_tmp}/sequential.log \
    || fail "release test-release-2409 from 01-app.yaml.gotmpl should be in output"

grep -q "test-infra-2409" ${issue_2409_tmp}/sequential.log \
    || fail "release test-infra-2409 from 02-infra.yaml.gotmpl should be in output"

grep -q "common-values-2409" ${issue_2409_tmp}/sequential.log \
    || fail "relative values file values/common.yaml should be resolved in sequential mode"

rm -rf ${issue_2409_tmp}

test_pass "issue 2409 sequential helmfiles with kubeContext"
