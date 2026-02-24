values {
  # Override with a literal - no dependency on image_version
  # Without proper DAG tracking of all definitions, this could cause
  # image_version to be evaluated after container_image if the override
  # is the only definition considered for dependency analysis
  container_image = "myapp:override"
}
