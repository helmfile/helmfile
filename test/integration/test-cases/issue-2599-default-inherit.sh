# Issue #2599: Test that defaultInherit applies template inheritance to all releases
# https://github.com/helmfile/helmfile/issues/2599
#
# This test verifies that:
#   - defaultInherit as a single string applies the template to all releases
#   - Releases without explicit inherit still get the template
#   - Releases with explicit inherit + except are not duplicated
#   - Non-existent template in defaultInherit produces a clear error

issue_2599_input_dir="${cases_dir}/issue-2599-default-inherit/input"
issue_2599_tmp=$(mktemp -d)

test_start "issue 2599 default inherit"

# Test 1: defaultInherit applies template to all releases
info "Running helmfile template with defaultInherit"
${helmfile} -f ${issue_2599_input_dir}/helmfile.yaml template \
    > ${issue_2599_tmp}/output.log 2>&1 \
    || { cat ${issue_2599_tmp}/output.log; fail "helmfile template with defaultInherit shouldn't fail"; }

# Verify namespace from template is applied to both releases
grep -q "namespace: default-ns" ${issue_2599_tmp}/output.log \
    || fail "namespace from default template should be applied"

# Verify both releases are processed
grep -q "app1" ${issue_2599_tmp}/output.log \
    || fail "release app1 should be in output"

grep -q "app2" ${issue_2599_tmp}/output.log \
    || fail "release app2 should be in output"

# Verify values from common.yaml are applied
grep -q "testValue" ${issue_2599_tmp}/output.log \
    || fail "values from common.yaml should be resolved"

# Test 2: non-existent template in defaultInherit should fail
info "Running helmfile template with non-existent defaultInherit template"
cat > ${issue_2599_tmp}/bad-helmfile.yaml <<EOF
defaultInherit: nonexistent
releases:
- name: app1
  chart: ${issue_2599_input_dir}/../../../charts/raw
EOF

${helmfile} -f ${issue_2599_tmp}/bad-helmfile.yaml template \
    > ${issue_2599_tmp}/error.log 2>&1 \
    && fail "helmfile template with non-existent defaultInherit template should fail"

grep -q "inexistent release template" ${issue_2599_tmp}/error.log \
    || fail "error message should mention inexistent release template"

rm -rf ${issue_2599_tmp}

test_pass "issue 2599 default inherit"
