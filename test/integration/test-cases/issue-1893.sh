issue_1893_input_dir="${cases_dir}/issue-1893/input"
helmfile_real="$(pwd)/${helmfile}"

test_start "issue 1893 helmfile template"
cd "${issue_1893_input_dir}"
${helmfile_real} template || fail "\"issue 1893 helmfile template shouldn't fail"
cd -
test_pass "issue 1893 helmfile template"