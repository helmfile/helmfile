issue_2451_input_dir="${cases_dir}/issue-2451-nested-helmfile-array-replace/input"
issue_2451_output_dir="${cases_dir}/issue-2451-nested-helmfile-array-replace/output"
issue_2451_tmp=$(mktemp -d)
issue_2451_result=${issue_2451_tmp}/result.yaml

test_start "issue 2451 - nested helmfile array replace (not merge)"
${helmfile} -f ${issue_2451_input_dir}/helmfile.yaml template --skip-deps > "${issue_2451_result}" || fail "\"helmfile template\" shouldn't fail"
./dyff between -bs "${issue_2451_output_dir}/output.yaml" "${issue_2451_result}" || fail "nested helmfile array values should replace default arrays entirely, not merge element-by-element"
test_pass "issue 2451 - nested helmfile array replace (not merge)"
