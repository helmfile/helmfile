# Issue #2269: helmDefaults.skipDeps and helmDefaults.skipRefresh should be respected
# https://github.com/helmfile/helmfile/issues/2269
#
# When helmDefaults.skipDeps=true and helmDefaults.skipRefresh=true are set in
# helmfile.yaml, helmfile should NOT attempt to add repos or update them.
# Previously, only the CLI flags --skip-deps and --skip-refresh worked;
# the helmDefaults equivalents were ignored in withPreparedCharts() and
# runHelmDepBuilds().

issue_2269_input_dir="${cases_dir}/issue-2269/input"
issue_2269_tmp=$(mktemp -d)
issue_2269_output="${issue_2269_tmp}/template.log"

test_start "issue-2269: helmDefaults.skipDeps and skipRefresh prevent repo operations"

# Run helmfile template WITHOUT any CLI --skip-deps/--skip-refresh flags.
# The helmDefaults in helmfile.yaml should be sufficient to skip repo operations.
# We use a fake repo URL that would fail if helmfile tried to contact it.
info "Running helmfile template with helmDefaults.skipDeps=true and skipRefresh=true"
${helmfile} -f "${issue_2269_input_dir}/helmfile.yaml" template > "${issue_2269_output}" 2>&1
code=$?

if [ $code -ne 0 ]; then
  cat "${issue_2269_output}"
  rm -rf "${issue_2269_tmp}"
  fail "helmfile template failed - helmDefaults.skipDeps/skipRefresh were likely ignored and helmfile tried to contact the fake repo"
fi

# Verify no repo operations occurred
if grep -q "Adding repo" "${issue_2269_output}"; then
  cat "${issue_2269_output}"
  rm -rf "${issue_2269_tmp}"
  fail "Issue #2269 regression: 'Adding repo' found in output despite helmDefaults.skipDeps=true and skipRefresh=true"
fi

info "No 'Adding repo' messages found"

if grep -q "Updating repo" "${issue_2269_output}" || grep -q "helm repo update" "${issue_2269_output}"; then
  cat "${issue_2269_output}"
  rm -rf "${issue_2269_tmp}"
  fail "Issue #2269 regression: repo update found in output despite helmDefaults.skipRefresh=true"
fi

info "No repo update messages found"

# Verify the template actually produced output (sanity check)
if ! grep -q "test-2269" "${issue_2269_output}"; then
  cat "${issue_2269_output}"
  rm -rf "${issue_2269_tmp}"
  fail "Template output missing expected content"
fi

info "Template produced expected output"

rm -rf "${issue_2269_tmp}"

test_pass "issue-2269: helmDefaults.skipDeps and skipRefresh prevent repo operations"
