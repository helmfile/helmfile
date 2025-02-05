v1_subhelmfile_multi_bases_with_array_values_input_dir="${cases_dir}/v1-subhelmfile-multi-bases-with-array-values/input"
v1_subhelmfile_multi_bases_with_array_values_output_dir="${cases_dir}/v1-subhelmfile-multi-bases-with-array-values/output"

yaml_overwrite_tmp=$(mktemp -d)
yaml_overwrite_reverse=${yaml_overwrite_tmp}/helmfile_template_result

v1_subhelmfile_multi_bases_with_array_values_output_file=${v1_subhelmfile_multi_bases_with_array_values_output_dir}/result
if [[ $EXTRA_HELMFILE_FLAGS == *--enable-live-output* ]]; then
    v1_subhelmfile_multi_bases_with_array_values_output_file=${v1_subhelmfile_multi_bases_with_array_values_output_dir}/result-live
fi

test_start "v1 subhelmfile multi bases with array values"
info "Comparing v1 subhelmfile multi bases with array values output ${yaml_overwrite_reverse} with ${v1_subhelmfile_multi_bases_with_array_values_output_file}"
for i in $(seq 10); do
    info "Comparing build/v1-subhelmfile-multi-bases-with-array-values #$i"
    ${helmfile} -f ${v1_subhelmfile_multi_bases_with_array_values_input_dir}/helmfile.yaml.gotmpl template -e dev &> ${yaml_overwrite_reverse} || fail "\"helmfile template\" shouldn't fail"
    diff -u ${v1_subhelmfile_multi_bases_with_array_values_output_file} ${yaml_overwrite_reverse} || fail "\"helmfile template\" should be consistent"
    echo code=$?
done
test_pass "v1 subhelmfile multi bases with array values"