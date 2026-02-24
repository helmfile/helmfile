values {
  # Base variable that will be referenced
  image_version = "1.0.0"

  # First definition depends on image_version
  container_image = "myapp:${hv.image_version}"
}
