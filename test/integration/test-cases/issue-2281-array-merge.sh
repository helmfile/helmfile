issue_2281_array_merge_input_dir="${cases_dir}/issue-2281-array-merge/input"
issue_2281_array_merge_output_dir="${cases_dir}/issue-2281-array-merge/output"

issue_2281_array_merge_tmp=$(mktemp -d)
issue_2281_array_merge_reverse=${issue_2281_array_merge_tmp}/issue.2281.array.merge.yaml

test_start "issue 2281 - array merge with state-values-set"
info "Comparing issue 2281 array merge output ${issue_2281_array_merge_reverse} with ${issue_2281_array_merge_output_dir}/output.yaml"

${helmfile} -f ${issue_2281_array_merge_input_dir}/helmfile.yaml.gotmpl template $(cat "$issue_2281_array_merge_input_dir/helmfile-extra-args") --skip-deps > "${issue_2281_array_merge_reverse}" || fail "\"helmfile template\" shouldn't fail"
./dyff between -bs "${issue_2281_array_merge_output_dir}/output.yaml" "${issue_2281_array_merge_reverse}" || fail "\"helmfile template\" output should match expected output - arrays should be merged element-by-element"

test_pass "issue 2281 - array merge with state-values-set"
