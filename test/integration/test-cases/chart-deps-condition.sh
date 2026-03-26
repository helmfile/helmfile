chart_deps_condition_input_dir="${cases_dir}/chart-deps-condition/input"

chart_deps_condition_tmp=$(mktemp -d)
actual=${chart_deps_condition_tmp}/actual.yaml
expected="${cases_dir}/chart-deps-condition/output/template"

test_start "chart with dependencies and condition"

# --concurrency 1 is a workaround for https://github.com/helmfile/helmfile/issues/2502
${helmfile} -f ${chart_deps_condition_input_dir}/helmfile.yaml template --skip-deps --concurrency 1 > ${actual} || fail "\"helmfile template\" shouldn't fail"
./dyff between -bs ${expected} ${actual} || fail "\"helmfile template\" should be consistent"

test_pass "chart with dependencies and condition"
