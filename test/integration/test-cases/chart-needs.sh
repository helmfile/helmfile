chart_need_case_input_dir="${cases_dir}/chart-needs/input"
chart_need_case_output_dir="${cases_dir}/chart-needs/output"

config_file="helmfile.yaml.gotmpl"
chart_needs_tmp=$(mktemp -d)
chart_needs_template_reverse=${chart_needs_tmp}/chart.needs.template.log
chart_needs_lint_reverse=${chart_needs_tmp}/chart.needs.lint.log
chart_needs_diff_reverse=${chart_needs_tmp}/chart.needs.diff.log

lint_out_file=${chart_need_case_output_dir}/lint
diff_out_file=${chart_need_case_output_dir}/diff
if [[ $EXTRA_HELMFILE_FLAGS == *--enable-live-output* ]]; then
    lint_out_file=${chart_need_case_output_dir}/lint-live
    diff_out_file=${chart_need_case_output_dir}/diff-live
fi

# Use Helm 4 variant files for lint (diff output is identical between Helm 3 and 4)
if [ "${HELMFILE_HELM4}" = "1" ]; then
    if [ -f "${lint_out_file}-helm4" ]; then
        lint_out_file="${lint_out_file}-helm4"
    fi
fi

test_start "chart prepare when helmfile template with needs"

info "https://github.com/helmfile/helmfile/issues/455"

for i in $(seq 10); do
    info "Comparing template/chart-needs #$i"
    ${helmfile} -f ${chart_need_case_input_dir}/${config_file} template --include-needs > ${chart_needs_template_reverse} || fail "\"helmfile template\" shouldn't fail"
    ./dyff between -bs ${chart_need_case_output_dir}/template ${chart_needs_template_reverse} || fail "\"helmfile template\" should be consistent"
done

for i in $(seq 10); do
    info "Comparing lint/chart-needs #$i"
    # Remove azuredisk-csi-driver repo to ensure consistent output (repo addition message)
    ${helm} repo remove azuredisk-csi-driver &>/dev/null || true
    ${helmfile} -f ${chart_need_case_input_dir}/${config_file} lint --include-needs | grep -v Linting | grep -v "has been removed" | grep -Ev "(Warning:.*is not a valid SemVerV2|\[WARNING\].*is not a valid SemVerV2|failed to load plugins)" > ${chart_needs_lint_reverse} || fail "\"helmfile lint\" shouldn't fail"
    diff -u ${lint_out_file} ${chart_needs_lint_reverse} || fail "\"helmfile lint\" should be consistent"
done

for i in $(seq 10); do
    info "Comparing diff/chart-needs #$i"
    # Remove azuredisk-csi-driver repo to ensure consistent output (repo addition message)
    ${helm} repo remove azuredisk-csi-driver &>/dev/null || true
    ${helmfile} -f ${chart_need_case_input_dir}/${config_file} diff --include-needs | grep -Ev "Comparing release=azuredisk-csi-storageclass, chart=.*/chartify.*/azuredisk-csi-storageclass" > ${chart_needs_diff_reverse}.tmp || fail "\"helmfile diff\" shouldn't fail"
    cat ${chart_needs_diff_reverse}.tmp | sed -E '/\*{20}/,/\*{20}/d' > ${chart_needs_diff_reverse}

    # With --enable-live-output, there's a race condition that can cause non-deterministic ordering
    # Try both the primary expected output and the alternate ordering
    if ! diff -u ${diff_out_file} ${chart_needs_diff_reverse} >/dev/null 2>&1; then
        if [[ $EXTRA_HELMFILE_FLAGS == *--enable-live-output* ]] && [ -f "${diff_out_file}-alt" ]; then
            info "Primary diff failed, trying alternate ordering (due to --enable-live-output race condition)"
            diff -u ${diff_out_file}-alt ${chart_needs_diff_reverse} || fail "\"helmfile diff\" should match either expected output"
        else
            diff -u ${diff_out_file} ${chart_needs_diff_reverse} || fail "\"helmfile diff\" should be consistent"
        fi
    fi
done

info "Applying ${chart_need_case_input_dir}/${config_file}"
${helmfile} -f ${chart_need_case_input_dir}/${config_file}  apply --include-needs
code=$?
[ ${code} -eq 0 ] || fail "unexpected exit code returned by helmfile apply: want 0, got ${code}"

${kubectl} get storageclass managed-csi -o yaml | grep -q "provisioner: disk.csi.azure.com" || fail "storageclass managed-csi should be created when applying helmfile.yaml"

info "Destroying ${chart_need_case_input_dir}/${config_file}"
${helmfile} -f ${chart_need_case_input_dir}/${config_file} destroy
code=$?
[ ${code} -eq 0 ] || fail "unexpected exit code returned by helmfile destroy: want 0, got ${code}"

info "Syncing ${chart_need_case_input_dir}/${config_file}"
${helmfile} -f ${chart_need_case_input_dir}/${config_file}  sync --include-needs
code=$?
[ ${code} -eq 0 ] || fail "unexpected exit code returned by helmfile apply: want 0, got ${code}"

${kubectl} get storageclass managed-csi -o yaml | grep -q "provisioner: disk.csi.azure.com" || fail "storageclass managed-csi should be created when syncing helmfile.yaml"

info "Destroying ${chart_need_case_input_dir}/${config_file}"
${helmfile} -f ${chart_need_case_input_dir}/${config_file} destroy
code=$?
[ ${code} -eq 0 ] || fail "unexpected exit code returned by helmfile destroy: want 0, got ${code}"

# Clean up: remove azuredisk-csi-driver repo to avoid conflicts with subsequent tests
${helm} repo remove azuredisk-csi-driver &>/dev/null || true

test_pass "chart prepare when helmfile template with needs"
