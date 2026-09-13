issue_2515_case_dir="$(cd "${cases_dir}/issue-2515" && pwd)"
issue_2515_tmp=$(mktemp -d)

# Determine the post-renderer argument.
# Helm 3 accepts an executable script; Helm 4 requires a plugin name.
if [ "${HELMFILE_HELM4}" = "1" ]; then
    test_start "issue-2515 post-renderer with output-dir-template (Helm 4)"
    info "Installing filter post-renderer plugin for Helm 4"
    ${helm} plugin uninstall filter &>/dev/null || true
    ${helm} plugin install ${issue_2515_case_dir}/input/helm-plugin-filter ${PLUGIN_INSTALL_FLAGS} || fail "Failed to install filter plugin"
    issue_2515_postrenderer_arg="filter"
else
    test_start "issue-2515 post-renderer with output-dir-template"
    issue_2515_postrenderer_arg="${issue_2515_case_dir}/input/filter.bash"
fi

info "Testing that --post-renderer output is written to files when --output-dir-template is set"

issue_2515_output_dir="${issue_2515_tmp}/output"

${helmfile} -f ${issue_2515_case_dir}/input/helmfile.yaml \
    template \
    --post-renderer ${issue_2515_postrenderer_arg} \
    --output-dir-template "${issue_2515_output_dir}/{{.Release.Name}}" \
    &> ${issue_2515_tmp}/log || fail "helmfile template should not fail"

if [ "${HELMFILE_HELM4}" = "1" ]; then
    # Helm 4 natively applies --post-renderer to --output-dir output.
    # The directory structure may differ from Helm 3 (no guaranteed templates/ subdir),
    # so search recursively for any YAML file. Fall back to stdout (log) if no files written.
    issue_2515_output_file=$(find "${issue_2515_output_dir}" -maxdepth 5 -type f \( -name '*.yaml' -o -name '*.yml' \) 2>/dev/null | head -n 1)
    if [ -z "${issue_2515_output_file}" ]; then
        # Helm 4 may write post-rendered output to stdout rather than files
        issue_2515_output_file="${issue_2515_tmp}/log"
        if ! grep -q "postrendered" "${issue_2515_output_file}"; then
            fail "Expected post-rendered YAML (namespace postrendered) in output files under ${issue_2515_output_dir} or stdout. Dir: $(find ${issue_2515_output_dir} 2>/dev/null || echo 'not found'). Log (last 50 lines): $(tail -50 ${issue_2515_output_file})"
        fi
    fi
else
    issue_2515_templates_dir="${issue_2515_output_dir}/issue-2515/templates"
    if [ ! -d "${issue_2515_templates_dir}" ]; then
        fail "Expected templates directory ${issue_2515_templates_dir} to exist"
    fi
    issue_2515_output_file=$(find "${issue_2515_templates_dir}" -type f \( -name '*.yaml' -o -name '*.yml' \) | head -n 1)
    if [ -z "${issue_2515_output_file}" ]; then
        fail "Expected rendered YAML file under ${issue_2515_templates_dir}"
    fi
fi

if grep -q "original-cm" "${issue_2515_output_file}"; then
    fail "Output should contain post-renderer output (Namespace), not original templates (original-cm). File contents: $(cat ${issue_2515_output_file})"
fi

if ! grep -q "postrendered" "${issue_2515_output_file}"; then
    fail "Output should contain post-renderer content (namespace postrendered). File contents: $(cat ${issue_2515_output_file})"
fi

if [ "${HELMFILE_HELM4}" = "1" ]; then
    test_pass "issue-2515 post-renderer with output-dir-template (Helm 4)"
else
    test_pass "issue-2515 post-renderer with output-dir-template"
fi
