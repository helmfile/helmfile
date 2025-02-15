postrender_case_input_dir="${cases_dir}/postrender/input"
postrender_case_output_dir="${cases_dir}/postrender/output"

config_file="helmfile.yaml.gotmpl"
postrender_diff_out_file=${postrender_case_output_dir}/diff-result
if [[ $EXTRA_HELMFILE_FLAGS == *--enable-live-output* ]]; then
    postrender_diff_out_file=${postrender_case_output_dir}/diff-result-live
fi

postrender_template_out_file=${postrender_case_output_dir}/template-result
if [[ $EXTRA_HELMFILE_FLAGS == *--enable-live-output* ]]; then
    postrender_template_out_file=${postrender_case_output_dir}/template-result-live
fi

postrender_diff_tmp=$(mktemp -d)
postrender_diff_reverse=${postrender_diff_tmp}/postrender.diff.build.yaml
postrender_template_reverse=${postrender_diff_tmp}/postrender.template.build.yaml

test_start "postrender diff"
info "Comparing postrender diff output ${postrender_diff_reverse} with ${postrender_case_output_dir}/result.yaml"
for i in $(seq 10); do
    info "Comparing build/postrender-diff #$i"
    ${helmfile} -f ${postrender_case_input_dir}/${config_file} diff --concurrency 1 --post-renderer ./add-cm.bash --post-renderer-args cm1 &> ${postrender_diff_reverse} || fail "\"helmfile diff\" shouldn't fail"
    diff -u  ${postrender_diff_out_file} ${postrender_diff_reverse} || fail "\"helmfile diff\" should be consistent"
    echo code=$?
done
test_pass "postrender diff"

test_start "postrender template"
info "Comparing postrender template output ${postrender_template_reverse} with ${postrender_case_output_dir}/result.yaml"
for i in $(seq 10); do
    info "Comparing build/postrender-diff #$i"
    ${helmfile} -f ${postrender_case_input_dir}/${config_file} template --concurrency 1 --post-renderer ./add-cm.bash --post-renderer-args cm1 &> ${postrender_template_reverse} || fail "\"helmfile template\" shouldn't fail"
    diff -u  ${postrender_template_out_file} ${postrender_template_reverse} || fail "\"helmfile template\" should be consistent"
    echo code=$?
done
test_pass "postrender template"
