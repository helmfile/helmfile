# Issue #2424: Test that --sequential-helmfiles resolves relative values file paths correctly
# https://github.com/helmfile/helmfile/issues/2424
#
# This test replicates a regression introduced in #2410 where a relative baseDir
# was passed instead of an absolute one, breaking values/secrets path resolution
# when using --sequential-helmfiles with helmfile.d/ containing multiple files.
#
# The setup mirrors the reporter's structure:
#   - helmfile.d/ with two .yaml.gotmpl files (to trigger multi-file sequential mode)
#   - bases/ with environments, defaults, and templates (values: ../config/{{ .Release.Namespace }}/values.yaml)
#   - Releases using inherit: [template: default] to pick up values via template
#   - config/<namespace>/values.yaml resolved via the template's relative path

issue_2424_input_dir="${cases_dir}/issue-2424-sequential-values-paths/input/helmfile.d"
issue_2424_tmp=$(mktemp -d)

test_start "issue 2424 sequential helmfiles values path resolution"

# Run template with --sequential-helmfiles to verify values paths resolve correctly
info "Running helmfile --sequential-helmfiles template"
${helmfile} --sequential-helmfiles -f ${issue_2424_input_dir} template \
    > ${issue_2424_tmp}/sequential.log 2>&1 \
    || fail "\"helmfile --sequential-helmfiles template\" shouldn't fail"

# Verify the values from config/default/values.yaml appear in template output
grep -q "app-configmap-2424" ${issue_2424_tmp}/sequential.log \
    || fail "values from ../config/default/values.yaml should be resolved in sequential mode"

grep -q "issue.*2424" ${issue_2424_tmp}/sequential.log \
    || fail "data from values file should appear in template output"

# Verify both releases are processed
grep -q "test-app-2424" ${issue_2424_tmp}/sequential.log \
    || fail "release test-app-2424 from 01-app.yaml.gotmpl should be in output"

grep -q "test-other-2424" ${issue_2424_tmp}/sequential.log \
    || fail "release test-other-2424 from 02-other.yaml.gotmpl should be in output"

# Run without --sequential-helmfiles and confirm same result
info "Running helmfile template without --sequential-helmfiles"
${helmfile} -f ${issue_2424_input_dir} template \
    > ${issue_2424_tmp}/parallel.log 2>&1 \
    || fail "\"helmfile template\" (parallel) shouldn't fail"

grep -q "app-configmap-2424" ${issue_2424_tmp}/parallel.log \
    || fail "values should resolve in parallel mode too"

rm -rf ${issue_2424_tmp}

test_pass "issue 2424 sequential helmfiles values path resolution"
