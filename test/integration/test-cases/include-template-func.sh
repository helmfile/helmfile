include_template_func_case_input_dir="${cases_dir}/include-template-func/input"
include_template_func_case_output_dir="${cases_dir}/include-template-func/output"

config_file="helmfile.yaml.gotmpl"

include_template_func_template_out_file=${include_template_func_case_output_dir}/template-result
if [[ $EXTRA_HELMFILE_FLAGS == *--enable-live-output* ]]; then
    include_template_func_template_out_file=${include_template_func_case_output_dir}/template-result-live
fi

include_template_func_template_tmp=$(mktemp -d)
include_template_func_template_reverse=${include_template_func_template_tmp}/include_template_func.template.build.yaml

test_start "include_template_func template"
info "Comparing include_template_func template output ${include_template_func_template_reverse} with ${include_template_func_case_output_dir}/result.yaml"
for i in $(seq 10); do
    info "Comparing build/include_template_func-template #$i"
    ${helmfile} -f ${include_template_func_case_input_dir}/${config_file} template --concurrency 1
    ${helmfile} -f ${include_template_func_case_input_dir}/${config_file} template --concurrency 1 &> ${include_template_func_template_reverse} || fail "\"helmfile template\" shouldn't fail"
    diff -u  ${include_template_func_template_out_file} ${include_template_func_template_reverse} || fail "\"helmfile template\" should be consistent"
done
test_pass "include_template_func template"
