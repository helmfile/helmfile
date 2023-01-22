buildin_statefile_case_input_dir="${cases_dir}/buildin-statefile/input"
buildin_statefile_case_output_dir="${cases_dir}/buildin-statefile/output"

buildin_statefile_tmp=$(mktemp -d)
buildin_statefile_reverse=${buildin_statefile_tmp}/buildin-statefile-template.yaml

test_start "buildin object statefile feature"
info "Comparing buildin object statefile feature output ${buildin_statefile_reverse} with ${buildin_statefile_case_output_dir}/statefile"
for i in $(seq 10); do
    info "Comparing template/buildin-statefile #$i"
    ${helmfile} -f ${buildin_statefile_case_input_dir}/root-helmfile.yaml.gotmpl template > ${buildin_statefile_reverse} || fail "\"helmfile template\" shouldn't fail"
    ./dyff between -bs ${buildin_statefile_case_output_dir}/statefile ${buildin_statefile_reverse} || fail "\"helmfile template\" should be consistent"
    echo code=$?
done
test_pass "buildin object statefile feature"
