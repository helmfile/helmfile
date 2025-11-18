yaml_kustomized_fetch_input_dir="${cases_dir}/kustomized-fetch/input"

test_start "kustomized fetch issue"
info "Checking kustomized fetch issue with ${yaml_kustomized_fetch_input_dir}/helmfile.yaml"

for i in $(seq 10); do
    info "checking kustomized fetch issue #$i"
    # Remove incubator repo to ensure clean state for each iteration
    ${helm} repo remove incubator 2>/dev/null || true
    kustomized_fetch_tmp=$(mktemp -d)
    ${helmfile} -f ${yaml_kustomized_fetch_input_dir}/helmfile.yaml fetch --output-dir ${kustomized_fetch_tmp} || fail "\"helmfile fetch\" shouldn't fail"
    rm -fr ${kustomized_fetch_tmp}
done
