values {
  base_var = "foundation"

  # This definition references base_var
  dependent = "prefix-${hv.base_var}"
}
