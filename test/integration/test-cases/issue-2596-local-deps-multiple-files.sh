# Integration test for issue #2596: Local dependencies with multiple release files
# https://github.com/helmfile/helmfile/issues/2596
# Reproduction: https://github.com/vgivanov/helmfile-deps-local-chart
#
# This test uses the exact same structure as the reproduction repo:
#   chart/Chart.yaml
#   helmfile.d/release1.yaml  (chart: ../chart with dependencies)
#   helmfile.d/release2.yaml  (chart: ../chart without dependencies)
#
# Before the fix, running `helmfile template` from this directory would fail with:
#   "failed reading adhoc dependencies: no helm list entry found for repository"
# because the relative dependency chart path "../chart" was not normalized against
# basePath before calling DirectoryExistsAt.

issue_2596_input_dir="${cases_dir}/issue-2596-local-deps-multiple-files/input"
issue_2596_tmp=""

cleanup_issue_2596() {
  if [ -n "${issue_2596_tmp}" ] && [ -d "${issue_2596_tmp}" ]; then
    rm -rf "${issue_2596_tmp}"
  fi
}
trap cleanup_issue_2596 EXIT

issue_2596_tmp=$(mktemp -d)
helmfile_real="$(pwd)/${helmfile}"

test_start "issue #2596: local deps with multiple release files"

info "Testing helmfile template with local chart dependencies across multiple release files"

cd "${issue_2596_input_dir}"

${helmfile_real} template > "${issue_2596_tmp}/output.yaml" 2>&1
result=$?

cd - > /dev/null

if [ $result -ne 0 ]; then
    cat "${issue_2596_tmp}/output.yaml"
    fail "helmfile template with local chart dependencies should not fail"
fi

info "Local chart dependencies with multiple release files works correctly"

cleanup_issue_2596
trap - EXIT

test_pass "issue #2596: local deps with multiple release files"
