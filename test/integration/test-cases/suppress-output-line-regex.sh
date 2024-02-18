suppress_output_line_regex_input_dir="${cases_dir}/suppress-output-line-regex/input"
suppress_output_line_regex_output_dir="${cases_dir}/suppress-output-line-regex/output"

suppress_output_line_regex_tmp=$(mktemp -d)
suppress_output_line_regex_reverse=${suppress_output_line_regex_tmp}/diff.args.build.yaml

case_title="suppress output line regex"
diff_out_file=${suppress_output_line_regex_output_dir}/diff
if [[ $EXTRA_HELMFILE_FLAGS == *--enable-live-output* ]]; then
    diff_out_file=${suppress_output_line_regex_output_dir}/diff-live
fi

if version_lt $HELM_DIFF_VERSION "3.9.0"; then
    echo "Skipping ${case_title} because helm-diff version is less than 3.9.0"
else
    test_start "$case_title"
    info "sync ${case_title} with default version"
    ${helmfile} -f ${suppress_output_line_regex_input_dir}/helmfile.yaml.gotmpl sync || fail "\"helmfile sync\" shouldn't fail"
    
    info "Comparing ${case_title} diff for output ${suppress_output_line_regex_reverse} with ${diff_out_file}"
    export SUPPRESS_OUTPUT_LINE_REGEX_INGRESS_NGINX_VERSION="4.9.0"
    
    for i in $(seq 10); do
        info "Comparing suppress-output-line-regex diff log #$i"
        ${helmfile} -f ${suppress_output_line_regex_input_dir}/helmfile.yaml.gotmpl diff > ${suppress_output_line_regex_reverse} || fail "\"helmfile diff\" shouldn't fail"
        diff -u ${diff_out_file} ${suppress_output_line_regex_reverse} || fail "\"helmfile diff\" should be consistent"
        echo code=$?
    done
    unset SUPPRESS_OUTPUT_LINE_REGEX_INGRESS_NGINX_VERSION
    
    echo "clean up ${case_title} resources"
    ${helmfile} -f ${suppress_output_line_regex_input_dir}/helmfile.yaml.gotmpl destroy || fail "\"helmfile destroy\" shouldn't fail"
    test_pass "$case_title"
fi