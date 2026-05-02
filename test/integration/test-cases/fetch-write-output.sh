fetch_write_output_input_dir="${cases_dir}/fetch-write-output/input"

fetch_write_output_tmp=$(mktemp -d)

case_title="fetch with --write-output for air-gapped environments"

test_start "$case_title"

info "Testing helmfile fetch --write-output with local chart"
output=$(${helmfile} -f ${fetch_write_output_input_dir}/helmfile.yaml.gotmpl fetch --output-dir ${fetch_write_output_tmp} --write-output 2>/dev/null) \
    || fail "\"helmfile fetch --write-output\" shouldn't fail"

info "Verifying stdout does not contain non-YAML status messages"
echo "${output}" | grep -q "^Charts will be downloaded to:" && fail "stdout should not contain 'Charts will be downloaded to:' (should be on stderr)" || true

info "Verifying output contains YAML document separator"
echo "${output}" | grep -q "^---" || fail "output should contain YAML document separator"

info "Verifying output contains source helmfile reference"
echo "${output}" | grep -q "#  Source:" || fail "output should contain source helmfile reference"

info "Verifying output contains release name"
echo "${output}" | grep -q "name: local-chart" || fail "output should contain release name"

info "Verifying output contains updated chart path pointing to output dir"
echo "${output}" | grep -q "chart:" || fail "output should contain chart field"

info "Verifying chart files exist in output directory"
cat ${fetch_write_output_tmp}/helmfile-tests/local-chart/raw/latest/Chart.yaml || fail "Chart.yaml should exist in fetched output directory"

info "Verifying the chart path in output matches the actual downloaded location"
chart_path=$(echo "${output}" | grep -E "^\s+(-\s+)?chart:" | head -1 | sed 's/.*chart: *//' | tr -d '"')
if [ ! -f "${chart_path}/Chart.yaml" ]; then
    fail "chart path '${chart_path}' from output should point to a directory containing Chart.yaml"
fi

rm -rf ${fetch_write_output_tmp}

test_pass "$case_title"
