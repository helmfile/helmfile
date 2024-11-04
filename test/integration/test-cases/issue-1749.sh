issue_1749_input_dir="${cases_dir}/issue-1749/input"
helmfile_real="$(pwd)/${helmfile}"

test_start "issue 1749 helmfile.d template --args --dry-run=server"
cd "${issue_1749_input_dir}"
${helmfile_real} template --args --dry-run=server || fail "\"issue 1749 helmfile.d template --args --dry-run=server\" shouldn't fail"
cd -
test_pass "issue 1749 helmfile.d template --args --dry-run=server"

test_start "issue 1749 helmfile.yaml template --args --dry-run=server"
${helmfile_real} template -f "${issue_1749_input_dir}/helmfile-2in1.yaml" --args --dry-run=server || fail "\"issue 1749 helmfile.d template --args --dry-run=server\" shouldn't fail"
test_pass "issue 1749 helmfile.yaml template --args --dry-run=server"
