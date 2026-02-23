values {
  # Override with a literal value (no dependency on base_var)
  # But the DAG must still know base_var needs to be evaluated before this
  dependent = "override-literal"
}
