chartify_case_input_dir="${cases_dir}/chartify/input"
chartify_case_output_dir="${cases_dir}/chartify/output"

config_file="helmfile.yaml.gotmpl"
chartify_tmp=$(mktemp -d)
chartify_template_reverse=${chartify_tmp}/chartify.template.log

test_start "helmfile template with chartify"

info "Comparing template/chartify"
${helmfile} -f ${chartify_case_input_dir}/${config_file} template >${chartify_template_reverse} || fail "\"helmfile template\" shouldn't fail"
./dyff between -bs ${chartify_case_output_dir}/template ${chartify_template_reverse} || fail "\"helmfile template\" should be consistent"

info "Comparing template/chartify with set"
${helmfile} -f ${chartify_case_input_dir}/${config_file} template --set image.tag=v2 >${chartify_template_reverse} || fail "\"helmfile template\" shouldn't fail"
./dyff between -bs ${chartify_case_output_dir}/template-set ${chartify_template_reverse} || fail "\"helmfile template\" should be consistent"

info "Comparing template/chartify with values"
${helmfile} -f ${chartify_case_input_dir}/${config_file} template --values "./extra-values.yaml" >${chartify_template_reverse} || fail "\"helmfile template\" shouldn't fail"
./dyff between -bs ${chartify_case_output_dir}/template-values ${chartify_template_reverse} || fail "\"helmfile template\" should be consistent"

test_pass "helmfile template with chartify"
