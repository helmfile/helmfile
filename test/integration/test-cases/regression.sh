
test_start "regression tests"

if [[ helm_major_version -eq 3 ]]; then
  regression_case_input_dir="${cases_dir}/regression/input"

  info "https://github.com/roboll/helmfile/issues/1857"
  config_file="issue.1857.yaml"
  if [[ ${HELMFILE_V1MODE} = true ]]; then
    pushd "${regression_case_input_dir}"
    mv "${config_file}" "${config_file}.gotmpl"
    config_file="${config_file}.gotmpl"
    popd
  fi
  (${helmfile} -f ${regression_case_input_dir}/${config_file} --state-values-set grafanaEnabled=true template | grep grafana 1>/dev/null) || fail "\"helmfile template\" shouldn't include grafana"
  ! (${helmfile} -f ${regression_case_input_dir}/${config_file} --state-values-set grafanaEnabled=false template | grep grafana) || fail "\"helmfile template\" shouldn't include grafana"

  info "https://github.com/roboll/helmfile/issues/1867"
  config_file="issue.1867.yaml"
  if [[ ${HELMFILE_V1MODE} = true ]]; then
    pushd "${regression_case_input_dir}"
    mv "${config_file}" "${config_file}.gotmpl"
    config_file="${config_file}.gotmpl"
    popd
  fi
  (${helmfile} -f ${regression_case_input_dir}/${config_file} template 1>/dev/null) || fail "\"helmfile template\" shouldn't fail"

  info "https://github.com/roboll/helmfile/issues/2118"
  config_file="issue.2118.yaml"
  if [[ ${HELMFILE_V1MODE} = true ]]; then
    pushd "${regression_case_input_dir}"
    mv "${config_file}" "${config_file}.gotmpl"
    config_file="${config_file}.gotmpl"
    popd
  fi
  (${helmfile} -f ${regression_case_input_dir}/${config_file} template 1>/dev/null) || fail "\"helmfile template\" shouldn't fail"
else
  info "There are no regression tests for helm 2 because all the target charts have dropped helm 2 support."
fi

test_pass "regression tests"