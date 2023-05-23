chartify_case_input_dir="${cases_dir}/chartify/input"
chartify_case_output_dir="${cases_dir}/chartify/output"

config_file="helmfile.yaml"
if [[ ${HELMFILE_V1MODE} = true ]]; then
  pushd "${chart_need_case_input_dir}"
  mv "${config_file}" "${config_file}.gotmpl"
  config_file="${config_file}.gotmpl"
  popd
fi

chartify_tmp=$(mktemp -d)
chartify_template_reverse=${chartify_tmp}/chartify.template.log


test_start "helmfile template with chartify"

info "Comparing template/chartify"
${helmfile} -f ${chartify_case_input_dir}/${config_file} template > ${chart_needs_template_reverse} || fail "\"helmfile template\" shouldn't fail"
./dyff between -bs ${chartify_case_output_dir}/template ${chart_needs_template_reverse} || fail "\"helmfile template\" should be consistent"

info "Comparing template/chartify with set"
${helmfile} -f ${chartify_case_input_dir}/${config_file} template --set image.tag=v2 > ${chart_needs_template_reverse} || fail "\"helmfile template\" shouldn't fail"
./dyff between -bs ${chartify_case_output_dir}/template-set ${chart_needs_template_reverse} || fail "\"helmfile template\" should be consistent"


test_pass "helmfile template with chartify"