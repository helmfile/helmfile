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
}
