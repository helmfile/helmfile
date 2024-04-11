
export VAULT_ADDR=http://127.0.0.1:8200
export VAULT_TOKEN=toor
info "Inject sops key"
sops="sops --hc-vault-transit $VAULT_ADDR/v1/sops/keys/key"

hcl_secrets_case_input_dir="${cases_dir}/hcl-secrets/input"
hcl_secrets_case_output_dir="${cases_dir}/hcl-secrets/output"

mkdir -p ${hcl_secrets_case_input_dir}/tmp

info "Encrypt secrets"
${sops} -e ${hcl_secrets_case_input_dir}/secrets.hcl > ${hcl_secrets_case_input_dir}/tmp/secrets.hcl || fail "${sops} failed at ${hcl_secrets_case_input_dir}/secrets.hcl"
${sops} -e ${hcl_secrets_case_input_dir}/secrets.yaml > ${hcl_secrets_case_input_dir}/tmp/secrets.yaml || fail "${sops} failed at ${hcl_secrets_case_input_dir}/secrets.yaml"


info "Ensure helm-secrets is installed"
${helm} plugin install https://github.com/jkroepke/helm-secrets --version v${HELM_SECRETS_VERSION}

info "values precedence order : yamlFile < hcl = hclSecrets  < secretYamlFile"
test_start "hcl-yaml-mix - should output secrets with proper overrides"

hcl_secrets_tmp=$(mktemp -d)
result=${hcl_secrets_tmp}/result.yaml

info "Building output"
${helmfile} -f ${hcl_secrets_case_input_dir}/_helmfile.yaml template --skip-deps > ${result} || fail "\"helmfile template\" shouldn't fail"
    diff -u ${hcl_secrets_case_output_dir}/output.yaml ${result} || fail "helmdiff should be consistent"
    echo code=$?

test_pass "hcl-yaml-mix"