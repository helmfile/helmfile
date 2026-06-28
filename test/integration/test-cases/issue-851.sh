#!/usr/bin/env bash

# Test for issue #851: Strategic merge with envelope charts does not work
# Issue: https://github.com/helmfile/helmfile/issues/851
#
# Problem: When an envelope chart's subchart has its own relative file:// dependencies
# (e.g. charts/sub2/Chart.yaml references file://../nested), helmfile's
# rewriteChartDependencies only rewrote the top-level Chart.yaml. The nested subchart's
# relative path was left untouched, so when chartify copied the chart into its temporary
# directory, the path resolved against the wrong location. The nested subchart silently
# failed to load and strategicMergePatches targeting resources defined in it errored with
# "no resource matches".
#
# Fix: rewriteChartDependencies recursively walks the chart tree and rewrites relative
# file:// dependencies in every Chart.yaml (top-level + nested subcharts).

issue_851_input_dir="${cases_dir}/issue-851/input"
issue_851_tmp_dir=$(mktemp -d)

cleanup_issue_851() {
  rm -rf "${issue_851_tmp_dir}"
}
trap cleanup_issue_851 EXIT

test_start "issue-851: strategic merge patches work with envelope charts"

info "Step 1: Templating envelope chart with nested subchart patches"
${helmfile} -f "${issue_851_input_dir}/helmfile.yaml" template > "${issue_851_tmp_dir}/templated.yaml" 2>&1
code=$?

if [ $code -ne 0 ]; then
  cat "${issue_851_tmp_dir}/templated.yaml"
  fail "helmfile template failed — envelope chart with nested subchart patches should succeed"
fi

info "✓ helmfile template succeeded"

# All three ServiceMonitors must be present in the output, including the one defined in
# the deepest nested subchart (sub2/charts/nested).
for expected_name in sub1-sm sub2-sm nested-sm; do
  if ! grep -q "name: ${expected_name}" "${issue_851_tmp_dir}/templated.yaml"; then
    cat "${issue_851_tmp_dir}/templated.yaml"
    fail "Expected ServiceMonitor ${expected_name} not found in templated output (envelope chart resources dropped)"
  fi
  info "✓ ServiceMonitor ${expected_name} present"
done

# Each ServiceMonitor must reflect its strategicMergePatch (patched port + interval).
if ! grep -q "port: metrics-sub1" "${issue_851_tmp_dir}/templated.yaml"; then
  cat "${issue_851_tmp_dir}/templated.yaml"
  fail "Patch for sub1-sm was not applied"
fi
info "✓ sub1-sm patch applied"

if ! grep -q "port: metrics-sub2" "${issue_851_tmp_dir}/templated.yaml"; then
  cat "${issue_851_tmp_dir}/templated.yaml"
  fail "Patch for sub2-sm was not applied"
fi
info "✓ sub2-sm patch applied"

# This is the key assertion for issue 851: the patch targeting the resource in the
# DEEPEST nested subchart (sub2/charts/nested) must be applied. Without the fix, the
# nested subchart's resources are dropped and this patch fails.
if ! grep -q "port: metrics-nested" "${issue_851_tmp_dir}/templated.yaml"; then
  cat "${issue_851_tmp_dir}/templated.yaml"
  fail "Patch for nested-sm was not applied — envelope chart nested subchart regression (issue 851)"
fi
info "✓ nested-sm patch applied (issue 851 fixed)"

cleanup_issue_851
trap - EXIT

test_pass "issue-851: strategic merge patches work with envelope charts"
