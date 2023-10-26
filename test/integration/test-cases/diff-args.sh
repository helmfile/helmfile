diff_args_input_dir="${cases_dir}/diff-args/input"
diff_args_output_dir="${cases_dir}/diff-args/output"

diff_args_tmp=$(mktemp -d)
diff_args_reverse=${diff_args_tmp}/diff.args.build.yaml

case_title="cli overwrite environment values"

test_start "$case_title"
info "Comparing ${case_title} for output ${diff_args_reverse} with ${diff_args_output_dir}/overwritten.yaml"
for i in $(seq 10); do
    info "Comparing diff-args diff debug log #$i"
    ${helmfile} -f ${diff_args_input_dir}/helmfile.yaml diff --debug > ${diff_args_reverse} || fail "\"helmfile template\" shouldn't fail"
    diff -u ${diff_args_output_dir}/diff.log ${diff_args_reverse} || fail "\"helmfile diff\" should be consistent"
    cat ${diff_args_reverse}
    echo code=$?
done
${helmfile} -f ${diff_args_input_dir}/helmfile.yaml apply --debug > ${diff_args_reverse} || fail "\"helmfile template\" shouldn't fail"
cat ${diff_args_reverse}
test_pass "cli overwrite environment values for v1"