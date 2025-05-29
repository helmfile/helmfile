chartify_with_non_chart_dirt_input_dir="${cases_dir}/chartify-with-non-chart-dir/input"
chartify_with_non_chart_dirt_output_dir="${cases_dir}/chartify-with-non-chart-dir/output"

chartify_with_non_chart_dirt_tmp=$(mktemp -d)
chartify_with_non_chart_dirt_reverse=${chartify_with_non_chart_dirt_tmp}/chartify.with.non.chart.build.yaml

case_title="chartify with non-chart dir"

diff_out_file=${chartify_with_non_chart_dirt_output_dir}/diff-result


if [[ $EXTRA_HELMFILE_FLAGS == *--enable-live-output* ]]; then
    diff_out_file=${chartify_with_non_chart_dirt_output_dir}/diff-result-live
fi

test_start "$case_title"
info "Comparing ${case_title} diff for output ${chartify_with_non_chart_dirt_reverse} with ${diff_out_file}"
for i in $(seq 10); do
    info "Comparing chartify-with-non-chart-dir diff log #$i"
    ${helmfile} -f ${chartify_with_non_chart_dirt_input_dir}/helmfiles/helmfile.yaml diff
    ${helmfile} -f ${chartify_with_non_chart_dirt_input_dir}/helmfiles/helmfile.yaml diff &> ${chartify_with_non_chart_dirt_reverse}.tmp || fail "\"helmfile diff\" shouldn't fail"
    cat ${chartify_with_non_chart_dirt_reverse}.tmp | grep -vE "^(Comparing release|Building dependency release)" > ${chartify_with_non_chart_dirt_reverse} 

    cat ${diff_out_file}
    cat ${chartify_with_non_chart_dirt_reverse}

    diff -u ${diff_out_file} ${chartify_with_non_chart_dirt_reverse} || fail "\"helmfile diff\" should be consistent"
    echo code=$?
done
test_pass "$case_title"