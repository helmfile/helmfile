# Integration test for issue #2502: Race condition when multiple releases share a local chart
# https://github.com/helmfile/helmfile/issues/2502
#
# When multiple releases reference the same local chart, concurrent goroutines
# race on rewriting Chart.yaml dependencies, causing:
#   "Error: validation: chart.metadata.name is required"
#
# This test verifies the fix works WITHOUT --concurrency 1 workaround.

issue_2502_input_dir="${cases_dir}/issue-2502-race-condition-local-chart/input"

issue_2502_tmp=$(mktemp -d)
actual="${issue_2502_tmp}/actual.yaml"

cleanup_issue_2502() {
  if [ -n "${issue_2502_tmp}" ] && [ -d "${issue_2502_tmp}" ]; then
    rm -rf "${issue_2502_tmp}"
  fi
}
trap cleanup_issue_2502 EXIT

test_start "issue #2502: race condition with shared local chart"

info "Running helmfile template with 5 releases sharing the same local chart (default concurrency)"

# Run WITHOUT --concurrency 1 to test the fix.
# Before the fix, this would intermittently fail with:
#   "Error: validation: chart.metadata.name is required"
# Run multiple iterations to increase chance of catching a race.
pass=0
iterations=5
for i in $(seq 1 ${iterations}); do
  if ${helmfile} -f ${issue_2502_input_dir}/helmfile.yaml -e test template > ${actual} 2>&1; then
    pass=$((pass + 1))
  else
    cat ${actual}
    fail "helmfile template failed on iteration ${i}/${iterations} (race condition on shared local chart)"
  fi
done

if [ ${pass} -ne ${iterations} ]; then
  fail "Expected ${iterations}/${iterations} passes but got ${pass}/${iterations}"
fi

info "All ${iterations} iterations passed successfully"

# Verify all 5 releases are present in output
for name in app app-2 app-3 app-4 app-5; do
  if ! grep -q "name: ${name}" ${actual}; then
    fail "Output should contain release '${name}'"
  fi
done

cleanup_issue_2502
trap - EXIT

test_pass "issue #2502: race condition with shared local chart"
