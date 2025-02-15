
test_start "regression tests"

regression_case_input_dir="${cases_dir}/regression/input"

info "https://github.com/roboll/helmfile/issues/1857"
config_file="issue.1857.yaml.gotmpl"
(${helmfile} -f ${regression_case_input_dir}/${config_file} --state-values-set grafanaEnabled=true template | grep grafana 1>/dev/null) || fail "\"helmfile template\" shouldn't include grafana"
! (${helmfile} -f ${regression_case_input_dir}/${config_file} --state-values-set grafanaEnabled=false template | grep grafana) || fail "\"helmfile template\" shouldn't include grafana"

info "https://github.com/roboll/helmfile/issues/1867"
config_file="issue.1867.yaml.gotmpl"
(${helmfile} -f ${regression_case_input_dir}/${config_file} template 1>/dev/null) || fail "\"helmfile template\" shouldn't fail"

info "https://github.com/roboll/helmfile/issues/2118"
config_file="issue.2118.yaml.gotmpl"
(${helmfile} -f ${regression_case_input_dir}/${config_file} template 1>/dev/null) || fail "\"helmfile template\" shouldn't fail"

info "https://github.com/helmfile/helmfile/issues/1682"
config_file="issue.1682.yaml.gotmpl"
(${helmfile} -f ${regression_case_input_dir}/${config_file} deps 1>/dev/null) || fail "\"helmfile deps\" shouldn't fail"

test_pass "regression tests"