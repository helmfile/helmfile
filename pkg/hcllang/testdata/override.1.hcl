locals {
  image = "v1.0"
}

values {
  env = "dev"
  config = {
    replicas = 1
    image    = local.image
  }
  tags = ["tag1", "tag2"]
}
