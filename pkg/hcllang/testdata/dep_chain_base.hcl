values {
  # This should be evaluated first
  version = "2.0"

  # This depends on version
  image = "app:${hv.version}"

  # This depends on image
  full_path = "registry/${hv.image}"
}
