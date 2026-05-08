postrender_defaults_args_case_input_dir="${cases_dir}/postrender-defaults-args/input"
postrender_defaults_args_case_output_dir="${cases_dir}/postrender-defaults-args/output"

# Helm 4 requires post-renderers to be plugins
if [ "${HELMFILE_HELM4}" = "1" ]; then
    info "Installing echo-args post-renderer plugin for Helm 4"
    ${helm} plugin uninstall echo-args &>/dev/null || true
    ${helm} plugin install ${postrender_defaults_args_case_input_dir}/helm-plugin-echo-args ${PLUGIN_INSTALL_FLAGS} || fail "Failed to install echo-args plugin"
fi

config_file="helmfile.yaml.gotmpl"
postrender_defaults_args_tmp=$(mktemp -d)

test_start "postrender-defaults-args template"
info "Running helmfile template with helmDefaults.postRendererArgs containing {{ .Release.Name }}"
${helmfile} -f ${postrender_defaults_args_case_input_dir}/${config_file} template --concurrency 1 &> ${postrender_defaults_args_tmp}/template.out || fail "\"helmfile template\" shouldn't fail"

info "Verifying that helmDefaults.postRendererArgs were templated with release names"
grep -q "name: rendered-arg-foo" ${postrender_defaults_args_tmp}/template.out || fail "Expected postRendererArg 'foo' for release foo, but not found in output"
grep -q "name: rendered-arg-bar" ${postrender_defaults_args_tmp}/template.out || fail "Expected postRendererArg 'bar' for release bar, but not found in output"

info "Verifying that literal template expression was NOT passed"
grep -q "rendered-arg-{{ .Release.Name }}" ${postrender_defaults_args_tmp}/template.out && fail "Template expression was NOT rendered (found literal {{ .Release.Name }})"

test_pass "postrender-defaults-args template"
