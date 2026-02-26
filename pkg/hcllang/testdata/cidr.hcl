values {
  host     = cidrhost("10.0.0.0/8", 2)
  host_neg = cidrhost("10.0.0.0/8", -2)
  netmask  = cidrnetmask("10.0.0.0/16")
  subnet   = cidrsubnet("10.0.0.0/8", 8, 2)
  subnets  = cidrsubnets("10.0.0.0/8", 4, 4)
}
