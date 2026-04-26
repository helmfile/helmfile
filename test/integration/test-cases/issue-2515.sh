issue_2515_case_dir="$(cd "${cases_dir}/issue-2515" && pwd)"
issue_2515_tmp=$(mktemp -d)

if [ "${HELMFILE_HELM4}" = "1" ]; then
    info "Skipping issue-2515 test for Helm 4 (Helm 4 natively applies --post-renderer to --output-dir output)"
    test_start "issue-2515 post-renderer with output-dir-template (skipped for Helm 4)"
    test_pass "issue-2515 post-renderer with output-dir-template (skipped for Helm 4)"
else
    test_start "issue-2515 post-renderer with output-dir-template"
    info "Testing that --post-renderer output is written to files when --output-dir-template is set"

    issue_2515_output_dir="${issue_2515_tmp}/output"

    ${helmfile} -f ${issue_2515_case_dir}/input/helmfile.yaml \
        template \
        --post-renderer ${issue_2515_case_dir}/input/filter.bash \
        --output-dir-template "${issue_2515_output_dir}/{{.Release.Name}}" \
        &> ${issue_2515_tmp}/log || fail "helmfile template should not fail"

    issue_2515_templates_dir="${issue_2515_output_dir}/issue-2515/templates"
    if [ ! -d "${issue_2515_templates_dir}" ]; then
        fail "Expected templates directory ${issue_2515_templates_dir} to exist"
    fi

    issue_2515_output_file=$(find "${issue_2515_templates_dir}" -type f \( -name '*.yaml' -o -name '*.yml' \) | head -n 1)
    if [ -z "${issue_2515_output_file}" ]; then
        fail "Expected rendered YAML file under ${issue_2515_templates_dir}"
    fi

    if grep -q "original-cm" "${issue_2515_output_file}"; then
        fail "Output should contain post-renderer output (Namespace), not original templates (original-cm). File contents: $(cat ${issue_2515_output_file})"
    fi

    if ! grep -q "postrendered" "${issue_2515_output_file}"; then
        fail "Output should contain post-renderer content (namespace postrendered). File contents: $(cat ${issue_2515_output_file})"
    fi

    test_pass "issue-2515 post-renderer with output-dir-template"
fi
