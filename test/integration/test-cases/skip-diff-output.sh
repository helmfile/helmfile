skip_diff_output_input_dir="${cases_dir}/skip-diff-output/input"
skip_diff_output_output_dir="${cases_dir}/skip-diff-output/output"

skip_diff_output_tmp=$(mktemp -d)
skip_diff_output_reverse=${skip_diff_output_tmp}/skip.diff.output.build.yaml

case_title="skip diff output"

diff_out_file=${skip_diff_output_output_dir}/diff-result
template_out_file=${skip_diff_output_output_dir}/template-result

if [[ $EXTRA_HELMFILE_FLAGS == *--enable-live-output* ]]; then
    diff_out_file=${skip_diff_output_output_dir}/diff-result-live
    template_out_file=${skip_diff_output_output_dir}/apply-result-live
fi

test_start "$case_title"
info "Comparing ${case_title} diff for output ${skip_diff_output_reverse} with ${diff_out_file}"
for i in $(seq 10); do
    info "Comparing skip-diff-output diff log #$i"
    ${helmfile} -f ${skip_diff_output_input_dir}/helmfile.yaml diff > ${skip_diff_output_reverse} || fail "\"helmfile diff\" shouldn't fail"
    diff -u ${diff_out_file} ${skip_diff_output_reverse} || fail "\"helmfile diff\" should be consistent"
    echo code=$?
done

info "Comparing ${case_title} template for output ${skip_diff_output_reverse} with ${template_out_file}"
for i in $(seq 10); do
    info "Comparing skip-diff-output template log #$i"
    ${helmfile} -f ${skip_diff_output_input_dir}/helmfile.yaml template > ${skip_diff_output_reverse} || fail "\"helmfile template\" shouldn't fail"
    diff -u ${template_out_file} ${skip_diff_output_reverse} || fail "\"helmfile template\" should be consistent"
    echo code=$?
done
test_pass "$case_title"