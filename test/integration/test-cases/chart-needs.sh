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

test_start "chart prepare when helmfile template with needs"

info "https://github.com/helmfile/helmfile/issues/455"

for i in $(seq 10); do
    info "Comparing template/chart-needs #$i"
    ${helmfile} -f ${chart_need_case_input_dir}/${config_file} template --include-needs > ${chart_needs_template_reverse} || fail "\"helmfile template\" shouldn't fail"
    ./dyff between -bs ${chart_need_case_output_dir}/template ${chart_needs_template_reverse} || fail "\"helmfile template\" should be consistent"
    echo code=$?
done

for i in $(seq 10); do
    info "Comparing lint/chart-needs #$i"
    ${helmfile} -f ${chart_need_case_input_dir}/${config_file} lint --include-needs | grep -v Linting > ${chart_needs_lint_reverse} || fail "\"helmfile lint\" shouldn't fail"
    diff -u ${lint_out_file} ${chart_needs_lint_reverse} || fail "\"helmfile lint\" should be consistent"
    echo code=$?
done

for i in $(seq 10); do
    info "Comparing diff/chart-needs #$i"
    ${helmfile} -f ${chart_need_case_input_dir}/${config_file} diff --include-needs | grep -Ev "Comparing release=azuredisk-csi-storageclass, chart=/tmp/.*/azuredisk-csi-storageclass" > ${chart_needs_diff_reverse} || fail "\"helmfile diff\" shouldn't fail"
    diff -u ${diff_out_file} ${chart_needs_diff_reverse} || fail "\"helmfile diff\" should be consistent"
    echo code=$?
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

test_pass "chart prepare when helmfile template with needs"