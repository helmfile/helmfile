test_start "key append feature"

key_append_case_input_dir="${cases_dir}/key_append"
key_append_expected_output="${key_append_case_input_dir}/output.yaml"

key_append_tmp=$(mktemp -d)
key_append_values_file="${key_append_tmp}/key_append.values.yaml"
key_append_generated_metrics_file="${key_append_tmp}/key_append_generated_metrics.yaml"

info "Testing key append functionality with nested structure"
config_file="helmfile.yaml.gotmpl"

info "Running helmfile template for key append test"
${helmfile} -f ${key_append_case_input_dir}/${config_file} template > ${key_append_values_file} || fail "\"helmfile template\" shouldn't fail"

info "Verifying that metricRelabelings+ is properly processed"
yq 'select(.metadata.name=="prometheus-monitoring-kube-state-metrics") | .spec.endpoints[].metricRelabelings' ${key_append_values_file} > ${key_append_generated_metrics_file}
./dyff between -bs ${key_append_expected_output} ${key_append_generated_metrics_file} || fail "\"helmfile template\" should be consistent"
echo code=$?
    
test_pass "key append feature" 
