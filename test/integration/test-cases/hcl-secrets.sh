
export VAULT_ADDR=http://127.0.0.1:8200
export VAULT_TOKEN=toor
info "Inject sops key"
sops="sops --hc-vault-transit $VAULT_ADDR/v1/sops/keys/key"

hcl_secrets_case_input_dir="${cases_dir}/hcl-secrets/input"
hcl_secrets_case_output_dir="${cases_dir}/hcl-secrets/output"

mkdir -p ${hcl_secrets_case_input_dir}/tmp

info "Ensure helm-secrets is installed"
# helm-secrets 4.7.0+ with Helm 4 uses split plugin architecture
# Helm 3 always uses single plugin installation regardless of helm-secrets version
if [[ "${HELMFILE_HELM4}" == "1" ]] && [[ "$(printf '%s\n' "4.7.0" "${HELM_SECRETS_VERSION}" | sort -V | head -n1)" == "4.7.0" ]]; then
    info "Installing helm-secrets v${HELM_SECRETS_VERSION} (split plugin architecture for Helm 4)"
    ${helm} plugin install https://github.com/jkroepke/helm-secrets/releases/download/v${HELM_SECRETS_VERSION}/helm-secrets.tgz ${PLUGIN_INSTALL_FLAGS} || true
    ${helm} plugin install https://github.com/jkroepke/helm-secrets/releases/download/v${HELM_SECRETS_VERSION}/helm-secrets-getter.tgz ${PLUGIN_INSTALL_FLAGS} || true
    ${helm} plugin install https://github.com/jkroepke/helm-secrets/releases/download/v${HELM_SECRETS_VERSION}/helm-secrets-post-renderer.tgz ${PLUGIN_INSTALL_FLAGS} || true
else
    info "Installing helm-secrets v${HELM_SECRETS_VERSION} (single plugin)"
    ${helm} plugin install https://github.com/jkroepke/helm-secrets --version v${HELM_SECRETS_VERSION} ${PLUGIN_INSTALL_FLAGS} || true
fi

info "Encrypt secrets"
${sops} -e ${hcl_secrets_case_input_dir}/secrets.hcl > ${hcl_secrets_case_input_dir}/tmp/secrets.hcl || fail "${sops} failed at ${hcl_secrets_case_input_dir}/secrets.hcl"
${sops} -e ${hcl_secrets_case_input_dir}/secrets.yaml > ${hcl_secrets_case_input_dir}/tmp/secrets.yaml || fail "${sops} failed at ${hcl_secrets_case_input_dir}/secrets.yaml"


info "values precedence order : yamlFile < hcl = hclSecrets  < secretYamlFile"
test_start "hcl-yaml-mix - should output secrets with proper overrides"

hcl_secrets_tmp=$(mktemp -d)
result=${hcl_secrets_tmp}/result.yaml

info "Building output"
${helmfile} -f ${hcl_secrets_case_input_dir}/_helmfile.yaml.gotmpl template --skip-deps > ${result} || fail "\"helmfile template\" shouldn't fail"
diff -u ${hcl_secrets_case_output_dir}/output.yaml ${result} || fail "helmdiff should be consistent"

test_pass "hcl-yaml-mix"
