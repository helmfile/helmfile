test_start "happypath - simple rollout of httpbin chart"

happypath_case_input_dir="${cases_dir}/happypath/input"
happypath_case_output_dir="${cases_dir}/happypath/output"

info "Diffing ${happypath_case_input_dir}/happypath.yaml"
bash -c "${helmfile} -f ${happypath_case_input_dir}/happypath.yaml diff --detailed-exitcode; code="'$?'"; [ "'${code}'" -eq 2 ]" || fail "unexpected exit code returned by helmfile diff"

info "Diffing ${happypath_case_input_dir}/happypath.yaml without color"
bash -c "${helmfile} -f ${happypath_case_input_dir}/happypath.yaml --no-color diff --detailed-exitcode; code="'$?'"; [ "'${code}'" -eq 2 ]" || fail "unexpected exit code returned by helmfile diff"

info "Diffing ${happypath_case_input_dir}/happypath.yaml with limited context"
bash -c "${helmfile} -f ${happypath_case_input_dir}/happypath.yaml diff --context 3 --detailed-exitcode; code="'$?'"; [ "'${code}'" -eq 2 ]" || fail "unexpected exit code returned by helmfile diff"

info "Diffing ${happypath_case_input_dir}/happypath.yaml with altered output"
bash -c "${helmfile} -f ${happypath_case_input_dir}/happypath.yaml diff --output simple --detailed-exitcode; code="'$?'"; [ "'${code}'" -eq 2 ]" || fail "unexpected exit code returned by helmfile diff"

info "Templating ${happypath_case_input_dir}/happypath.yaml"
rm -rf ${dir}/tmp
${helmfile} -f ${happypath_case_input_dir}/happypath.yaml --debug template --output-dir tmp
code=$?
[ ${code} -eq 0 ] || fail "unexpected exit code returned by helmfile template: ${code}"
for output in $(ls -d ${dir}/tmp/*); do
    # e.g. test/integration/tmp/happypath-877c0dd4-helmx/helmx
    for release_dir in $(ls -d ${output}/*); do
        release_name=$(basename ${release_dir})
        golden_dir=${happypath_case_output_dir}/v${helm_major_version}/${release_name}
        info "Comparing template output ${release_dir}/templates with ${golden_dir}"
        ./diff-yamls ${golden_dir} ${release_dir}/templates || fail "unexpected diff in template result for ${release_name}"
    done
done

info "Applying ${happypath_case_input_dir}/happypath.yaml"
bash -c "${helmfile} -f ${happypath_case_input_dir}/happypath.yaml apply --detailed-exitcode; code="'$?'"; echo Code: "'$code'"; [ "'${code}'" -eq 2 ]" || fail "unexpected exit code returned by helmfile apply"

info "Syncing ${happypath_case_input_dir}/happypath.yaml"
${helmfile} -f ${happypath_case_input_dir}/happypath.yaml sync
wait_deploy_ready httpbin-httpbin
retry 5 "curl --fail $(minikube service --url --namespace=${test_ns} httpbin-httpbin)/status/200"
[ ${retry_result} -eq 0 ] || fail "httpbin failed to return 200 OK"

info "Applying ${happypath_case_input_dir}/happypath.yaml"
${helmfile} -f ${happypath_case_input_dir}/happypath.yaml apply --detailed-exitcode
code=$?
[ ${code} -eq 0 ] || fail "unexpected exit code returned by helmfile apply: want 0, got ${code}"

info "Locking dependencies"
${helmfile} -f ${happypath_case_input_dir}/happypath.yaml deps
code=$?
[ ${code} -eq 0 ] || fail "unexpected exit code returned by helmfile deps: ${code}"

info "Applying ${happypath_case_input_dir}/happypath.yaml with locked dependencies"
${helmfile} -f ${happypath_case_input_dir}/happypath.yaml apply
code=$?
[ ${code} -eq 0 ] || fail "unexpected exit code returned by helmfile apply: ${code}"
${helm} list --namespace=${test_ns} || fail "unable to list releases"

info "Deleting release"
${helmfile} -f ${happypath_case_input_dir}/happypath.yaml delete
${helm} status --namespace=${test_ns} httpbin &> /dev/null && fail "release should not exist anymore after a delete"

info "Ensuring \"helmfile delete\" doesn't fail when no releases installed"
${helmfile} -f ${happypath_case_input_dir}/happypath.yaml delete || fail "\"helmfile delete\" shouldn't fail when there are no installed releases"

info "Ensuring \"helmfile template\" output does contain only YAML docs"
(${helmfile} -f ${happypath_case_input_dir}/happypath.yaml template | kubectl apply -f -) || fail "\"helmfile template | kubectl apply -f -\" shouldn't fail"

test_pass "happypath"