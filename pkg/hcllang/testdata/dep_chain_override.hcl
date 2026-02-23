values {
  # Override full_path with a literal (no dependencies)
  # But image still depends on version, so without tracking all definitions,
  # the DAG might not properly order version -> image -> full_path
  full_path = "override/path"
}
