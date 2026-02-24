locals {
  image = "v1.0"
}

values {
  env = "dev"
  config = {
    replicas = 1
    image    = local.image
  }
  tags  = ["tag1", "tag2"]
  class = "standard"
  annotations = {
    a = "val1"
    b = "val2"
  }

  mixed_types = {
    string_value = "string"
    number_value = 42
    bool_value   = true
    list_value   = ["item1", "item2"]
  }
}
