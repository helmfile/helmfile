chartify_case_input_dir="${cases_dir}/chartify/input"
chartify_case_output_dir="${cases_dir}/chartify/output"

config_file="helmfile.yaml.gotmpl"
chartify_tmp=$(mktemp -d)
chartify_template_reverse=${chartify_tmp}/chartify.template.log

# Use Helm 4 variant files if available (output format may differ)
template_out_file="${chartify_case_output_dir}/template"
template_set_out_file="${chartify_case_output_dir}/template-set"
template_values_out_file="${chartify_case_output_dir}/template-values"

if [ "${HELMFILE_HELM4}" = "1" ]; then
    if [ -f "${template_out_file}-helm4" ]; then
        template_out_file="${template_out_file}-helm4"
    fi
    if [ -f "${template_set_out_file}-helm4" ]; then
        template_set_out_file="${template_set_out_file}-helm4"
    fi
    if [ -f "${template_values_out_file}-helm4" ]; then
        template_values_out_file="${template_values_out_file}-helm4"
    fi
fi

test_start "helmfile template with chartify"

info "Comparing template/chartify"
${helmfile} -f ${chartify_case_input_dir}/${config_file} template >${chartify_template_reverse} || fail "\"helmfile template\" shouldn't fail"
./dyff between -bs ${template_out_file} ${chartify_template_reverse} || fail "\"helmfile template\" should be consistent"

info "Comparing template/chartify with set"
${helmfile} -f ${chartify_case_input_dir}/${config_file} template --set image.tag=v2 >${chartify_template_reverse} || fail "\"helmfile template\" shouldn't fail"
./dyff between -bs ${template_set_out_file} ${chartify_template_reverse} || fail "\"helmfile template\" should be consistent"

info "Comparing template/chartify with values"
${helmfile} -f ${chartify_case_input_dir}/${config_file} template --values "./extra-values.yaml" >${chartify_template_reverse} || fail "\"helmfile template\" shouldn't fail"
./dyff between -bs ${template_values_out_file} ${chartify_template_reverse} || fail "\"helmfile template\" should be consistent"

test_pass "helmfile template with chartify"
