export VAULT_ADDR=http://127.0.0.1:8200
export VAULT_TOKEN=toor
sops="sops --hc-vault-transit $VAULT_ADDR/v1/sops/keys/key"

secretssops_case_input_dir="${cases_dir}/secretssops/input"
secretssops_case_output_dir="${cases_dir}/secretssops/output"
config_file="secretssops.yaml.gotmpl"

mkdir -p ${secretssops_case_input_dir}/tmp

info "Encrypt secrets"
${sops} -e ${secretssops_case_input_dir}/env-1.secrets.yaml > ${secretssops_case_input_dir}/tmp/env-1.secrets.sops.yaml || fail "${sops} failed at ${secretssops_case_input_dir}/env-1.secrets.yaml"
${sops} -e ${secretssops_case_input_dir}/env-2.secrets.yaml > ${secretssops_case_input_dir}/tmp/env-2.secrets.sops.yaml || fail "${sops} failed at ${secretssops_case_input_dir}/env-2.secrets.yaml"

test_start "secretssops.1 - should fail without secrets plugin"

info "Ensure helm-secrets is not installed"
${helm} plugin rm secrets || true

info "Ensure helmfile fails when no helm-secrets is installed"
unset code
${helmfile} -f ${secretssops_case_input_dir}/${config_file} -e direct build || code="$?"; code="${code:-0}"
echo Code: "${code}"
[ "${code}" -ne 0 ] || fail "\"helmfile build\" should fail without secrets plugin"

test_pass "secretssops.1"

test_start "secretssops.2 - should succeed with secrets plugin"

info "Ensure helm-secrets is installed"
${helm} plugin install https://github.com/jkroepke/helm-secrets --version v${HELM_SECRETS_VERSION}

info "Ensure helmfile succeed when helm-secrets is installed"
${helmfile} -f ${secretssops_case_input_dir}/${config_file} -e direct build || fail "\"helmfile build\" shouldn't fail"

test_pass "secretssops.2"

test_start "secretssops.3 - should order secrets correctly"

secretssops_tmp=$(mktemp -d)
direct=${secretssops_tmp}/direct.build.yaml
reverse=${secretssops_tmp}/reverse.build.yaml

info "Building secrets output"

info "Comparing build/direct output ${direct} with ${secretssops_case_output_dir}"
for i in $(seq 10); do
    info "Comparing build/direct #$i"
    ${helmfile} -f ${secretssops_case_input_dir}/${config_file} -e direct template --skip-deps > ${direct} || fail "\"helmfile template\" shouldn't fail"
    ./dyff between -bs ${secretssops_case_output_dir}/direct.build.yaml ${direct} || fail "\"helmfile template\" should be consistent"
    echo code=$?
done

info "Comparing build/reverse output ${direct} with ${secretssops_case_output_dir}"
for i in $(seq 10); do
    info "Comparing build/reverse #$i"
    ${helmfile} -f ${secretssops_case_input_dir}/${config_file} -e reverse template --skip-deps > ${reverse} || fail "\"helmfile template\" shouldn't fail"
    ./dyff between -bs ${secretssops_case_output_dir}/reverse.build.yaml ${reverse} || fail "\"helmfile template\" should be consistent"
    echo code=$?
done

test_pass "secretssops.3"