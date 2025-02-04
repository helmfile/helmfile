yaml_overwrite_case_input_dir="${cases_dir}/yaml-overwrite/input"
yaml_overwrite_case_output_dir="${cases_dir}/yaml-overwrite/output"

yaml_overwrite_tmp=$(mktemp -d)
yaml_overwrite_reverse=${yaml_overwrite_tmp}/yaml.override.build.yaml

test_start "yaml overwrite feature"
info "Comparing yaml overwrite feature output ${yaml_overwrite_reverse} with ${yaml_overwrite_case_output_dir}/overwritten.yaml"
for i in $(seq 10); do
    info "Comparing build/yaml-overwrite #$i"
    ${helmfile} -f ${yaml_overwrite_case_input_dir}/issue.657.yaml.gotmpl template --skip-deps > ${yaml_overwrite_reverse} || fail "\"helmfile template\" shouldn't fail"
    ./dyff between -bs ${yaml_overwrite_case_output_dir}/overwritten.yaml ${yaml_overwrite_reverse} || fail "\"helmfile template\" should be consistent"
    echo code=$?
done
test_pass "yaml overwrite feature"