issue_2515_case_dir="${cases_dir}/issue-2515"
issue_2515_tmp=$(mktemp -d)

if [ "${HELMFILE_HELM4}" = "1" ]; then
    info "Skipping issue-2515 test for Helm 4 (post-renderer requires plugin)"
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

    issue_2515_output_file="${issue_2515_output_dir}/issue-2515/issue-2515.yaml"
    if [ ! -f "${issue_2515_output_file}" ]; then
        fail "Expected output file ${issue_2515_output_file} to exist"
    fi

    if grep -q "original-cm" "${issue_2515_output_file}"; then
        fail "Output should contain post-renderer output (Namespace), not original templates (original-cm). File contents: $(cat ${issue_2515_output_file})"
    fi

    if ! grep -q "postrendered" "${issue_2515_output_file}"; then
        fail "Output should contain post-renderer content (namespace postrendered). File contents: $(cat ${issue_2515_output_file})"
    fi

    test_pass "issue-2515 post-renderer with output-dir-template"
fi
