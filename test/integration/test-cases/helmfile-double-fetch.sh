helmfile_double_fetch_case_input_dir="${cases_dir}/helmfile-double-fetch/input"

config_file="helmfile.yaml"

test_start "helmfile fetch with helmfile_double_fetch"

info "Comparing fetch/helmfile_double_fetch_first"
${helmfile} -f ${helmfile_double_fetch_case_input_dir}/${config_file} fetch --output-dir /tmp/chartdir || fail "\"helmfile fetch\" shouldn't fail"

info "Comparing template/helmfile_double_fetch_second"
${helmfile} -f ${helmfile_double_fetch_case_input_dir}/${config_file} fetch --output-dir /tmp/chartdir || fail "\"helmfile fetch\" shouldn't fail"

test_pass "helmfile fetch with helmfile_double_fetch"