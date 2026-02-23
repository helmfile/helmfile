locals {
  image = "v1.1"
}

values {
  env     = "prod"
  country = "us"
  config = {
    replicas = 3
    image    = local.image
    debug    = true
  }
  class  = null
  region = "${hv.country}-east"
  status = upper("ready")
  annotations = {
    a = "val2"
    b = "val3"
  }

  mixed_types = {
    string_value = false
    number_value = ["val1", "val2"]
    bool_value   = 1
    list_value   = "item1"
  }
}
