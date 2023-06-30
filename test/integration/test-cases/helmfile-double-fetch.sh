helmfile_double_fetch_case_input_dir="${cases_dir}/helmfile_double_fetch/input"
helmfile_double_fetch_case_output_dir="${cases_dir}/helmfile_double_fetch/output"

config_file="helmfile.yaml"

helmfile_double_fetch_tmp=$(mktemp -d)
helmfile_double_fetch_template_reverse=${helmfile_double_fetch_tmp}/helmfile_double_fetch.template.log


test_start "helmfile fetch with helmfile_double_fetch"

info "Comparing fetch/helmfile_double_fetch_first"
${helmfile} -f ${helmfile_double_fetch_case_input_dir}/${config_file} fetch --debug --output-dir /tmp/chartdir 2>&1 | grep -v `pwd` > ${helmfile_double_fetch_template_reverse} || fail "\"helmfile fetch\" shouldn't fail"
diff -u ${helmfile_double_fetch_case_output_dir}/fetch_first ${helmfile_double_fetch_template_reverse} || fail "\"helmfile fetch\" should be consistent"

info "Comparing template/helmfile_double_fetch_second"
${helmfile} -f ${helmfile_double_fetch_case_input_dir}/${config_file} fetch --debug --output-dir /tmp/chartdir 2>&1 | grep -v `pwd` > ${helmfile_double_fetch_template_reverse} || fail "\"helmfile template\" shouldn't fail"
diff -u ${helmfile_double_fetch_case_output_dir}/fetch_second ${helmfile_double_fetch_template_reverse} || fail "\"helmfile fetch\" should be consistent"


test_pass "helmfile fetch with helmfile_double_fetch"