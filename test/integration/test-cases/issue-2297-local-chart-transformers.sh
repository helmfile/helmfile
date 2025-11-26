# Integration test for issue #2297: Local chart + transformers causes panic
# https://github.com/helmfile/helmfile/issues/2297
#
# This test verifies that local charts with relative paths (like "../chart")
# work correctly when transformers are present. The bug manifests when there
# are MULTIPLE release files in helmfile.d and one uses local chart + transformers.
# Before the fix, chartify would try "helm pull ../chart" which fails because
# the path wasn't normalized.

issue_2297_input_dir="${cases_dir}/issue-2297-local-chart-transformers/input"
issue_2297_tmp=$(mktemp -d)
# Convert helmfile to absolute path before cd (otherwise ./helmfile won't be found)
helmfile_real="$(pwd)/${helmfile}"

test_start "issue #2297: local chart with transformers"

info "Testing helmfile template with local chart and transformers"

# Run from the input directory where helmfile.d/ contains the helmfile with "../chart" reference
cd "${issue_2297_input_dir}"

# This should succeed - before the fix it would fail with:
# "helm pull ../chart --untar" fails with "repo .. not found"
${helmfile_real} template > "${issue_2297_tmp}/output.yaml" 2>&1
result=$?

cd - > /dev/null

if [ $result -ne 0 ]; then
    cat "${issue_2297_tmp}/output.yaml"
    fail "helmfile template with local chart and transformers should not fail"
fi

# Verify the output contains the expected configmap with transformer annotation
if ! grep -q "test-annotation" "${issue_2297_tmp}/output.yaml"; then
    cat "${issue_2297_tmp}/output.yaml"
    fail "Output should contain the transformer annotation"
fi

info "Local chart with transformers works correctly"

test_pass "issue #2297: local chart with transformers"
