#!/usr/bin/env bash

# Test for issue #851: Strategic merge with envelope charts does not work
# Issue: https://github.com/helmfile/helmfile/issues/851
#
# Problem: When an envelope chart's subchart declares its own local file://
# dependency, helm's `dependency build` on the parent chart fetches the subchart
# and packages it as .tgz, but does NOT recursively build the subchart's own
# deps first. The resulting .tgz lacks the nested subchart's resources. chartify's
# helm template silently drops those resources, and strategicMergePatches/jsonPatches
# targeting them fail with "no resource matches".
#
# Fix: preBuildTransitiveSubchartDeps walks the transitive file:// dep tree and
# runs `helm dependency build` bottom-up on each subchart source dir before
# chartify runs, so every subchart .tgz includes its own nested deps.

issue_851_input_dir="${cases_dir}/issue-851/input"

# Copy the chart tree to a temp dir so the test doesn't leave generated
# Chart.lock / charts/*.tgz files in the repo. preBuildTransitiveSubchartDeps
# runs `helm dependency build` on subchart source directories, which creates
# those artifacts. The test must not pollute the source tree.
issue_851_tmp_dir=$(mktemp -d)
cp -r "${issue_851_input_dir}/." "${issue_851_tmp_dir}/"

cleanup_issue_851() {
  rm -rf "${issue_851_tmp_dir}"
}
trap cleanup_issue_851 EXIT

test_start "issue-851: strategic merge patches work with envelope charts"

info "Step 1: Templating envelope chart whose subchart has its own file:// dep"
${helmfile} -f "${issue_851_tmp_dir}/helmfile.yaml" template > "${issue_851_tmp_dir}/templated.yaml" 2>&1
code=$?

if [ $code -ne 0 ]; then
  cat "${issue_851_tmp_dir}/templated.yaml"
  fail "helmfile template failed — envelope chart with nested subchart patches should succeed"
fi

info "✓ helmfile template succeeded"

# All three ServiceMonitors must be present, including the one defined in sub2's
# OWN dependency (nested). Without the fix, nested-sm is silently dropped.
for expected_name in sub1-sm sub2-sm nested-sm; do
  if ! grep -q "name: ${expected_name}" "${issue_851_tmp_dir}/templated.yaml"; then
    cat "${issue_851_tmp_dir}/templated.yaml"
    fail "Expected ServiceMonitor ${expected_name} not found (envelope chart nested subchart resources dropped)"
  fi
  info "✓ ServiceMonitor ${expected_name} present"
done

# Each ServiceMonitor must reflect its strategicMergePatch.
for expected_port in metrics-sub1 metrics-sub2 metrics-nested; do
  if ! grep -q "port: ${expected_port}" "${issue_851_tmp_dir}/templated.yaml"; then
    cat "${issue_851_tmp_dir}/templated.yaml"
    fail "Patch targeting ${expected_port} was not applied (issue 851 regression)"
  fi
done
info "✓ all strategicMergePatches applied (sub1-sm, sub2-sm, nested-sm)"

# The critical assertion: nested-sm's patch applied. Without the fix, the nested
# subchart's resources are missing and this patch fails with "no resource matches".
if ! grep -q "port: metrics-nested" "${issue_851_tmp_dir}/templated.yaml"; then
  cat "${issue_851_tmp_dir}/templated.yaml"
  fail "Patch for nested-sm was not applied — envelope chart regression (issue 851)"
fi
info "✓ nested-sm patch applied (issue 851 fixed)"

cleanup_issue_851
trap - EXIT

test_pass "issue-851: strategic merge patches work with envelope charts"
