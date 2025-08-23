deps_kustomization_i_1402="${cases_dir}/deps-kustomization-i-1402/input"

config_file="helmfile.yaml"

test_start "helmfile deps nonreg for #1402"

${helmfile} -f ${deps_kustomization_i_1402}/${config_file} deps || fail "\"helmfile deps\" shouldn't fail"

test_pass "helmfile deps nonreg for #1402"
