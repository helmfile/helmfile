postrender_case_input_dir="${cases_dir}/postrender/input"
postrender_case_output_dir="${cases_dir}/postrender/output"

# Helm 4 requires post-renderers to be plugins, not executable scripts
if [ "${HELMFILE_HELM4}" = "1" ]; then
    info "Installing add-cm post-renderer plugin for Helm 4"
    # Remove plugin if already exists, then install
    ${helm} plugin uninstall add-cm &>/dev/null || true
    ${helm} plugin install ${postrender_case_input_dir}/helm-plugin-add-cm ${PLUGIN_INSTALL_FLAGS} || fail "Failed to install add-cm plugin"
    postrenderer_arg="add-cm"
else
    postrenderer_arg="./add-cm.bash"
fi

config_file="helmfile.yaml.gotmpl"
postrender_diff_out_file=${postrender_case_output_dir}/diff-result
if [[ $EXTRA_HELMFILE_FLAGS == *--enable-live-output* ]]; then
    postrender_diff_out_file=${postrender_case_output_dir}/diff-result-live
fi

postrender_template_out_file=${postrender_case_output_dir}/template-result
if [[ $EXTRA_HELMFILE_FLAGS == *--enable-live-output* ]]; then
    postrender_template_out_file=${postrender_case_output_dir}/template-result-live
fi

# Use Helm 4 variant files for postrender (output format differs)
if [ "${HELMFILE_HELM4}" = "1" ]; then
    if [ -f "${postrender_diff_out_file}-helm4" ]; then
        postrender_diff_out_file="${postrender_diff_out_file}-helm4"
    fi
    if [ -f "${postrender_template_out_file}-helm4" ]; then
        postrender_template_out_file="${postrender_template_out_file}-helm4"
    fi
fi

postrender_diff_tmp=$(mktemp -d)
postrender_diff_reverse=${postrender_diff_tmp}/postrender.diff.build.yaml
postrender_template_reverse=${postrender_diff_tmp}/postrender.template.build.yaml

test_start "postrender diff"
info "Comparing postrender diff output ${postrender_diff_reverse} with ${postrender_case_output_dir}/result.yaml"
for i in $(seq 10); do
    info "Comparing build/postrender-diff #$i"
    ${helmfile} -f ${postrender_case_input_dir}/${config_file} diff --concurrency 1 --post-renderer ${postrenderer_arg} --post-renderer-args cm1 &> ${postrender_diff_reverse}.tmp || fail "\"helmfile diff\" shouldn't fail"
    cat ${postrender_diff_reverse}.tmp | sed -E '/\*{20}/,/\*{20}/d' > ${postrender_diff_reverse}
    diff -u  ${postrender_diff_out_file} ${postrender_diff_reverse} || fail "\"helmfile diff\" should be consistent"
done
test_pass "postrender diff"

test_start "postrender template"
info "Comparing postrender template output ${postrender_template_reverse} with ${postrender_case_output_dir}/result.yaml"
for i in $(seq 10); do
    info "Comparing build/postrender-diff #$i"
    ${helmfile} -f ${postrender_case_input_dir}/${config_file} template --concurrency 1 --post-renderer ${postrenderer_arg} --post-renderer-args cm1 &> ${postrender_template_reverse} || fail "\"helmfile template\" shouldn't fail"
    diff -u  ${postrender_template_out_file} ${postrender_template_reverse} || fail "\"helmfile template\" should be consistent"
done
test_pass "postrender template"
