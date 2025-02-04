chartify_jsonPatches_and_strategicMergePatches_case_input_dir="${cases_dir}/chartify-jsonPatches-and-strategicMergePatches/input"
chartify_jsonPatches_and_strategicMergePatches_case_output_dir="${cases_dir}/chartify-jsonPatches-and-strategicMergePatches/output"

config_file="helmfile.yaml.gotmpl"
chartify_jsonPatches_and_strategicMergePatches_tmp=$(mktemp -d)
chartify_jsonPatches_and_strategicMergePatches_template_reverse=${chartify_jsonPatches_and_strategicMergePatches_tmp}/chartify_jsonPatches_and_strategicMergePatches.template.log


test_start "helmfile template with chartify_jsonPatches_and_strategicMergePatches"

info "Comparing template/chartify_jsonPatches_and_strategicMergePatches"
${helmfile} -f ${chartify_jsonPatches_and_strategicMergePatches_case_input_dir}/${config_file} template > ${chartify_jsonPatches_and_strategicMergePatches_template_reverse} || fail "\"helmfile template\" shouldn't fail"
cat ${chartify_jsonPatches_and_strategicMergePatches_template_reverse}
./dyff between -bs ${chartify_jsonPatches_and_strategicMergePatches_case_output_dir}/template ${chartify_jsonPatches_and_strategicMergePatches_template_reverse} || fail "\"helmfile template\" should be consistent"

test_pass "helmfile template with chartify_jsonPatches_and_strategicMergePatches"