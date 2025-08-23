cli_overwrite_environment_values_input_dir="${cases_dir}/cli-overwrite-environment-values/input"
cli_overwrite_environment_values_output_dir="${cases_dir}/cli-overwrite-environment-values/output"

cli_overwrite_environment_values_tmp=$(mktemp -d)
cli_overwrite_environment_values_reverse=${cli_overwrite_environment_values_tmp}/cli.environment.override.build.yaml

case_title="cli overwrite environment values"

test_start "$case_title"
info "Comparing ${case_title} for output ${cli_overwrite_environment_values_reverse} with ${cli_overwrite_environment_values_output_dir}/overwritten.yaml"
for i in $(seq 10); do
    info "Comparing build/cli-overwrite-environment-values #$i"
    ${helmfile} -f ${cli_overwrite_environment_values_input_dir}/input.yaml.gotmpl template --state-values-set ns=test3 --state-values-set-string imageTag=1.23.3,zone="zone1,zone2" > ${cli_overwrite_environment_values_reverse} || fail "\"helmfile template\" shouldn't fail"
    diff -u ${cli_overwrite_environment_values_output_dir}/output.yaml ${cli_overwrite_environment_values_reverse} || fail "\"helmfile template\" should be consistent"
    echo code=$?
done
test_pass "cli overwrite environment values for v1"