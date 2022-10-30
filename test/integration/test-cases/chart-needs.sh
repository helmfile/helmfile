if [[ helm_major_version -eq 3 ]]; then
  chart_needs_tmp=$(mktemp -d)
  chart_needs_golden_dir=${dir}/chart-needs-golden
  chart_needs_template_reverse=${chart_needs_tmp}/chart.needs.template.log
  chart_needs_lint_reverse=${chart_needs_tmp}/chart.needs.lint.log
  chart_needs_diff_reverse=${chart_needs_tmp}/chart.needs.diff.log

  test_start "chart prepare when helmfile template with needs"

  info "https://github.com/helmfile/helmfile/issues/455"

  for i in $(seq 10); do
      info "Comparing template/chart-needs #$i"
      ${helmfile} -f ${dir}/issue.455/helmfile.yaml template --include-needs > ${chart_needs_template_reverse} || fail "\"helmfile template\" shouldn't fail"
      ./yamldiff ${chart_needs_golden_dir}/template ${chart_needs_template_reverse} || fail "\"helmfile template\" should be consistent"
      echo code=$?
  done

  for i in $(seq 10); do
      info "Comparing lint/chart-needs #$i"
      ${helmfile_no_extra_flags} -f ${dir}/issue.455/helmfile.yaml lint --include-needs | grep -v Linting > ${chart_needs_lint_reverse} || fail "\"helmfile lint\" shouldn't fail"
      diff -u ${chart_needs_golden_dir}/lint ${chart_needs_lint_reverse} || fail "\"helmfile lint\" should be consistent"
      echo code=$?
  done

  for i in $(seq 10); do
      info "Comparing diff/chart-needs #$i"
      ${helmfile_no_extra_flags} -f ${dir}/issue.455/helmfile.yaml diff --include-needs | grep -Ev "Comparing release=azuredisk-csi-storageclass, chart=/tmp/[0-9a-zA-Z]+/azuredisk-csi-storageclass" | grep -v "$test_ns" > ${chart_needs_diff_reverse} || fail "\"helmfile diff\" shouldn't fail"
      diff -u ${chart_needs_golden_dir}/diff ${chart_needs_diff_reverse} || fail "\"helmfile diff\" should be consistent"
      echo code=$?
  done

  info "Applying ${dir}/issue.455/helmfile.yaml"
  ${helmfile} -f ${dir}/issue.455/helmfile.yaml  apply --include-needs
  code=$?
  [ ${code} -eq 0 ] || fail "unexpected exit code returned by helmfile apply: want 0, got ${code}"

  ${kubectl} get storageclass managed-csi -o yaml | grep -q "provisioner: disk.csi.azure.com" || fail "storageclass managed-csi should be created when applying helmfile.yaml"

  info "Destroying ${dir}/issue.455/helmfile.yaml"
  ${helmfile} -f ${dir}/issue.455/helmfile.yaml destroy
  code=$?
  [ ${code} -eq 0 ] || fail "unexpected exit code returned by helmfile destroy: want 0, got ${code}"

  info "Syncing ${dir}/issue.455/helmfile.yaml"
  ${helmfile} -f ${dir}/issue.455/helmfile.yaml  sync --include-needs
  code=$?
  [ ${code} -eq 0 ] || fail "unexpected exit code returned by helmfile apply: want 0, got ${code}"

  ${kubectl} get storageclass managed-csi -o yaml | grep -q "provisioner: disk.csi.azure.com" || fail "storageclass managed-csi should be created when syncing helmfile.yaml"

  info "Destroying ${dir}/issue.455/helmfile.yaml"
  ${helmfile} -f ${dir}/issue.455/helmfile.yaml destroy
  code=$?
  [ ${code} -eq 0 ] || fail "unexpected exit code returned by helmfile destroy: want 0, got ${code}"

  test_pass "chart prepare when helmfile template with needs"
fi