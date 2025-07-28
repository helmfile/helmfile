fetch_forl_local_chart_input_dir="${cases_dir}/fetch-forl-local-chart/input"

fetch_forl_local_chart_tmp=$(mktemp -d)

case_title="fetch for local chart"

test_start "$case_title"

info "Comparing fetch-forl-local-chart diff log #$i"
${helmfile} -f ${fetch_forl_local_chart_input_dir}/helmfile.yaml.gotmpl fetch --output-dir ${fetch_forl_local_chart_tmp} || fail "\"helmfile fetch\" shouldn't fail"
cat ${fetch_forl_local_chart_tmp}/local-chart/local-chart/raw/latest/Chart.yaml || fail "Chart.yaml should exist in the fetched local chart directory"
echo code=$?

test_pass "$case_title"