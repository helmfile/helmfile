state_values_set_cli_args_in_environments_input_dir="${cases_dir}/state-values-set-cli-args-in-environments/input"
state_values_set_cli_args_in_environments_output_dir="${cases_dir}/state-values-set-cli-args-in-environments/output"

state_values_set_cli_args_in_environments_tmp=$(mktemp -d)
state_values_set_cli_args_in_environments_reverse=${state_values_set_cli_args_in_environments_tmp}/state.values.set.cli.args.build.yaml

test_start "state values set cli args in environments"
info "Comparing state values set cli args environments output ${state_values_set_cli_args_in_environments_reverse} with ${state_values_set_cli_args_in_environments_output_dir}/output.yaml"

${helmfile} -f ${state_values_set_cli_args_in_environments_input_dir}/helmfile.yaml.gotmpl template  $(cat "$state_values_set_cli_args_in_environments_input_dir/helmfile-extra-args") --skip-deps > "${state_values_set_cli_args_in_environments_reverse}" || fail "\"helmfile template\" shouldn't fail"
./dyff between -bs "${state_values_set_cli_args_in_environments_output_dir}/output.yaml" "${state_values_set_cli_args_in_environments_reverse}" || fail "\"helmfile template\" should be consistent"
echo code=$?

test_pass "state values set cli args in environments"
