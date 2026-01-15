# Issue #2309: Test that helm template receives --kube-context when using jsonPatches
# https://github.com/helmfile/helmfile/issues/2309
#
# This test verifies that helmfile correctly passes --kube-context to helm template
# when helmDefaults.kubeContext is set and jsonPatches are used.

issue_2309_case_input_dir="${cases_dir}/issue-2309-kube-context-template/input"
issue_2309_case_output_dir="${cases_dir}/issue-2309-kube-context-template/output"

config_file="helmfile.yaml.gotmpl"
issue_2309_tmp=$(mktemp -d)
issue_2309_template_output=${issue_2309_tmp}/issue_2309.template.log

test_start "helmfile template with kube-context and jsonPatches (issue #2309)"

info "Running helmfile template with kubeContext and jsonPatches"
${helmfile} -f ${issue_2309_case_input_dir}/${config_file} template > ${issue_2309_template_output} || fail "\"helmfile template\" shouldn't fail"

info "Checking template output"
cat ${issue_2309_template_output}

# Verify the output contains the patched ConfigMap
if grep -q "patched: \"true\"" ${issue_2309_template_output}; then
    info "Found patched value in output - jsonPatches applied successfully"
else
    fail "jsonPatches were not applied - missing 'patched: true' in output"
fi

# Compare with expected output
./dyff between -bs ${issue_2309_case_output_dir}/template ${issue_2309_template_output} || fail "\"helmfile template\" output should match expected"

test_pass "helmfile template with kube-context and jsonPatches (issue #2309)"
