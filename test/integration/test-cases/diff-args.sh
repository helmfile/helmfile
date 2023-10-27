diff_args_input_dir="${cases_dir}/diff-args/input"
diff_args_output_dir="${cases_dir}/diff-args/output"

diff_args_tmp=$(mktemp -d)
diff_args_reverse=${diff_args_tmp}/diff.args.build.yaml

case_title="diff args"
diff_out_file=${diff_args_output_dir}/diff
apply_out_file=${diff_args_output_dir}/apply
if [[ $EXTRA_HELMFILE_FLAGS == *--enable-live-output* ]]; then
    apply_out_file=${diff_args_output_dir}/apply-live
    diff_out_file=${diff_args_output_dir}/diff-live
fi

test_start "$case_title"
info "Comparing ${case_title} for output ${diff_args_reverse} with ${diff_out_file}"
for i in $(seq 10); do
    info "Comparing diff-args diff debug log #$i"
    ${helmfile} -f ${diff_args_input_dir}/helmfile.yaml diff > ${diff_args_reverse} || fail "\"helmfile diff\" shouldn't fail"
    diff -u ${diff_out_file} ${diff_args_reverse} || fail "\"helmfile diff\" should be consistent"
    echo code=$?
done
${helmfile} -f ${diff_args_input_dir}/helmfile.yaml apply > ${diff_args_reverse} || fail "\"helmfile apply\" shouldn't fail"
cat ${diff_args_reverse}
diff -u ${apply_out_file} ${diff_args_reverse} || fail "\"helmfile apply\" should be consistent"
test_pass "$case_title"