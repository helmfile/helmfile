diff_args_input_dir="${cases_dir}/diff-args/input"
diff_args_output_dir="${cases_dir}/diff-args/output"

diff_args_tmp=$(mktemp -d)
diff_args_reverse=${diff_args_tmp}/diff.args.build.yaml
diff_args_reverse_stderr=${diff_args_tmp}/diff.args.build.stderr.yaml

case_title="diff args"
diff_out_file=${diff_args_output_dir}/diff
apply_out_file=${diff_args_output_dir}/apply
diff_out_stderr_file=${diff_args_output_dir}/diff-stderr
apply_out_stderr_file=${diff_args_output_dir}/apply-stderr
if [[ $EXTRA_HELMFILE_FLAGS == *--enable-live-output* ]]; then
    diff_out_file=${diff_args_output_dir}/diff-live
    apply_out_file=${diff_args_output_dir}/apply-live
    diff_out_stderr_file=${diff_args_output_dir}/diff-live-stderr
    apply_out_stderr_file=${diff_args_output_dir}/apply-live-stderr
fi

test_start "$case_title"
info "Comparing ${case_title} diff for output ${diff_args_reverse} with ${diff_out_file}"
info "Comparing ${case_title} diff for output ${diff_args_reverse_stderr} with ${diff_out_stderr_file}"
for i in $(seq 10); do
    info "Comparing diff-args diff log #$i"
    ${helmfile} -f ${diff_args_input_dir}/helmfile.yaml diff 1> ${diff_args_reverse}.tmp 2> ${diff_args_reverse_stderr} || fail "\"helmfile diff\" shouldn't fail"
    cat ${diff_args_reverse}.tmp | sed -E '/\*{20}/,/\*{20}/d' > ${diff_args_reverse}
    diff -u ${diff_out_file} ${diff_args_reverse} || fail "\"helmfile diff\" should be consistent"
    diff -u ${diff_out_stderr_file} ${diff_args_reverse_stderr} || fail "\"helmfile diff\" should be consistent (stderr)"
    echo code=$?
done
info "Comparing ${case_title} apply for output ${diff_args_reverse} with ${apply_out_file}"
info "Comparing ${case_title} apply for stdout ${diff_args_reverse_stderr} with ${apply_out_stderr_file}"
${helmfile} -f ${diff_args_input_dir}/helmfile.yaml apply 1> ${diff_args_reverse} 2> ${diff_args_reverse_stderr} || fail "\"helmfile apply\" shouldn't fail"
diff -u ${apply_out_file} <(grep -vE "^(LAST DEPLOYED|installed)" ${diff_args_reverse}) || fail "\"helmfile apply\" should be consistent"
diff -u ${apply_out_stderr_file} <(grep -vE "^(LAST DEPLOYED|installed)" ${diff_args_reverse_stderr}) || fail "\"helmfile apply\" should be consistent (stderr)"
echo "clean up diff args resources"
${helmfile} -f ${diff_args_input_dir}/helmfile.yaml destroy || fail "\"helmfile destroy\" shouldn't fail"
test_pass "$case_title"