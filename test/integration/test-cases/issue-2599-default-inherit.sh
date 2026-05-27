# Issue #2599: Test that defaultInherit applies template inheritance to all releases
# https://github.com/helmfile/helmfile/issues/2599
#
# This test verifies that:
#   - defaultInherit as a single string applies the template to all releases
#   - Releases without explicit inherit still get the template
#   - Releases with explicit inherit + except are not duplicated
#   - Non-existent template in defaultInherit produces a clear error

issue_2599_input_dir="${cases_dir}/issue-2599-default-inherit/input"
issue_2599_tmp=""

cleanup_issue_2599() {
  if [ -n "${issue_2599_tmp}" ] && [ -d "${issue_2599_tmp}" ]; then
    rm -rf "${issue_2599_tmp}"
  fi
}
trap cleanup_issue_2599 EXIT

issue_2599_tmp=$(mktemp -d)

test_start "issue 2599 default inherit"

# Test 1: defaultInherit applies template to all releases
info "Running helmfile build with defaultInherit"
${helmfile} -f "${issue_2599_input_dir}/helmfile.yaml" build \
    > "${issue_2599_tmp}/output.log" 2>&1 \
    || { cat "${issue_2599_tmp}/output.log"; fail "helmfile build with defaultInherit shouldn't fail"; }

# Verify namespace from template is applied to both releases
grep -q "namespace: default-ns" "${issue_2599_tmp}/output.log" \
    || fail "namespace from default template should be applied"

# Verify both releases are processed
grep -q "app1" "${issue_2599_tmp}/output.log" \
    || fail "release app1 should be in output"

grep -q "app2" "${issue_2599_tmp}/output.log" \
    || fail "release app2 should be in output"
grep -q "^templates:" "${issue_2599_tmp}/output.log" \
    || fail "templates section should be in build output"

# Verify inherited values and labels per release
sed -n '/name: app1/,/name: app2/p' "${issue_2599_tmp}/output.log" > "${issue_2599_tmp}/app1.log"
sed -n '/name: app2/,/^templates:/{/^templates:/!p}' "${issue_2599_tmp}/output.log" > "${issue_2599_tmp}/app2.log"
[ -s "${issue_2599_tmp}/app1.log" ] || fail "failed to extract release app1 section from build output"
[ -s "${issue_2599_tmp}/app2.log" ] || fail "failed to extract release app2 section from build output"

grep -Eq 'managed:[[:space:]]*"?true"?([[:space:]]|$)' "${issue_2599_tmp}/app1.log" \
    || fail "release app1 should inherit managed label from default template"
grep -q "common.yaml" "${issue_2599_tmp}/app1.log" \
    || fail "release app1 should inherit values from common.yaml"
grep -q "common.yaml" "${issue_2599_tmp}/app2.log" \
    || fail "release app2 should inherit values from common.yaml"
if grep -Eq 'managed:[[:space:]]*"?true"?([[:space:]]|$)' "${issue_2599_tmp}/app2.log"; then
    fail "release app2 should not inherit managed label due to except"
fi

# Test 2: non-existent template in defaultInherit should fail
info "Running helmfile build with non-existent defaultInherit template"
cat > "${issue_2599_tmp}/bad-helmfile.yaml" <<EOF
defaultInherit: nonexistent
releases:
- name: app1
  chart: ${dir}/charts/raw
EOF

${helmfile} -f "${issue_2599_tmp}/bad-helmfile.yaml" build \
    > "${issue_2599_tmp}/error.log" 2>&1 \
    && fail "helmfile build with non-existent defaultInherit template should fail"

grep -q "inexistent release template" "${issue_2599_tmp}/error.log" \
    || fail "error message should mention inexistent release template"

test_pass "issue 2599 default inherit"
