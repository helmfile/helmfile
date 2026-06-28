# Integration test for issue #1880: Kustomize transformers preventing dependency update
# https://github.com/helmfile/helmfile/issues/1880
#
# This test verifies that a local chart with a relative file:// dependency
# (file://../library) works correctly when transformers are used.
#
# Before the fix, chartify copied the chart to a temp directory, breaking the
# relative file:// path. helm dependency up then failed with:
#   Error: directory /tmp/chartify.../monitoring/library not found

issue_1880_input_dir="${cases_dir}/issue-1880-transformers-with-file-deps/input"
issue_1880_tmp=""

cleanup_issue_1880() {
  if [ -n "${issue_1880_tmp}" ] && [ -d "${issue_1880_tmp}" ]; then
    rm -rf "${issue_1880_tmp}"
  fi
}
trap cleanup_issue_1880 EXIT

issue_1880_tmp=$(mktemp -d)
helmfile_real="$(pwd)/${helmfile}"

test_start "issue #1880: transformers with relative file:// chart dependencies"

info "Testing helmfile template with transformers and file:// dependency"

cd "${issue_1880_input_dir}"

${helmfile_real} template > "${issue_1880_tmp}/output.yaml" 2>&1
result=$?

cd - > /dev/null

if [ $result -ne 0 ]; then
    cat "${issue_1880_tmp}/output.yaml"
    fail "helmfile template should not fail (issue #1880 regression)"
fi

# Verify the transformer was applied (LabelTransformer adds component=abc)
if ! grep -q "component: abc" "${issue_1880_tmp}/output.yaml"; then
    cat "${issue_1880_tmp}/output.yaml"
    fail "Output should contain the transformer label 'component: abc'"
fi

info "Transformers with relative file:// dependencies work correctly"

cleanup_issue_1880
trap - EXIT

test_pass "issue #1880: transformers with relative file:// chart dependencies"
