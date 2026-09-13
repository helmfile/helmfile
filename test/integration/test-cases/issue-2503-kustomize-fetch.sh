issue_2503_input_dir="${cases_dir}/issue-2503-kustomize-fetch/input"

test_start "issue-2503 helmfile fetch with kustomization directory"

info "Testing helmfile fetch with local kustomization directory (issue #2503)"

for i in $(seq 3); do
    info "Testing helmfile fetch with kustomization #$i"
    issue_2503_tmp=$(mktemp -d)
    ${helmfile} -f ${issue_2503_input_dir}/helmfile.yaml fetch --output-dir ${issue_2503_tmp} || fail "\"helmfile fetch\" shouldn't fail with kustomization directory"
    rm -fr ${issue_2503_tmp}
done

test_pass "issue-2503 helmfile fetch with kustomization directory"
