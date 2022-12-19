if [[ helm_major_version -eq 3 ]]; then
  postrender_diff_case_input_dir="${cases_dir}/postrender-diff/input"
  postrender_diff_case_output_dir="${cases_dir}/postrender-diff/output"

  postrender_diff_out_file=${postrender_diff_case_output_dir}/result
  if [[ $EXTRA_HELMFILE_FLAGS == *--enable-live-output* ]]; then
      postrender_diff_out_file=${postrender_diff_case_output_dir}/result-live
  fi

  postrender_diff_tmp=$(mktemp -d)
  postrender_diff_reverse=${postrender_diff_tmp}/postrender.diff.build.yaml

  test_start "postrender diff"
  info "Comparing postrender diff output ${postrender_diff_reverse} with ${postrender_diff_case_output_dir}/result.yaml"
  for i in $(seq 10); do
      info "Comparing build/postrender-diff #$i"
      ${helmfile} -f ${postrender_diff_case_input_dir}/helmfile.yaml diff --concurrency 1 &> ${postrender_diff_reverse} || fail "\"helmfile diff\" shouldn't fail"
      diff -u  ${postrender_diff_out_file} ${postrender_diff_reverse} || fail "\"helmfile diff\" should be consistent"
      echo code=$?
  done
  test_pass "postrender diff"
fi