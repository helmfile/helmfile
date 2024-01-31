deps_mr_1011_input_dir="${cases_dir}/deps-mr-1011/input"

config_file="helmfile.yaml"

test_start "helmfile deps nonreg for #1011"

${helmfile} -f ${deps_mr_1011_input_dir}/${config_file} deps || fail "\"helmfile deps\" shouldn't fail"

test_pass "helmfile deps nonreg for #1011"
