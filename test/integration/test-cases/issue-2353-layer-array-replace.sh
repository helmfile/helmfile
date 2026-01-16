issue_2353_layer_array_replace_input_dir="${cases_dir}/issue-2353-layer-array-replace/input"
issue_2353_layer_array_replace_output_dir="${cases_dir}/issue-2353-layer-array-replace/output"

issue_2353_layer_array_replace_tmp=$(mktemp -d)
issue_2353_layer_array_replace_reverse=${issue_2353_layer_array_replace_tmp}/issue.2353.layer.array.replace.yaml

test_start "issue 2353 - layer array replace (not merge)"
info "Comparing issue 2353 layer array replace output ${issue_2353_layer_array_replace_reverse} with ${issue_2353_layer_array_replace_output_dir}/output.yaml"

${helmfile} -f ${issue_2353_layer_array_replace_input_dir}/helmfile.yaml.gotmpl template $(cat "$issue_2353_layer_array_replace_input_dir/helmfile-extra-args") --skip-deps > "${issue_2353_layer_array_replace_reverse}" || fail "\"helmfile template\" shouldn't fail"
./dyff between -bs "${issue_2353_layer_array_replace_output_dir}/output.yaml" "${issue_2353_layer_array_replace_reverse}" || fail "\"helmfile template\" output should match expected output - environment values arrays should replace default arrays entirely"

test_pass "issue 2353 - layer array replace (not merge)"
