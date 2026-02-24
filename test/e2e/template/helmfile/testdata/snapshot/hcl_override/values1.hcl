locals {
  filename = "values1.hcl"
}

values {
  a = local.filename
  b = "b"
  c = "${hv.a}${hv.b}"
  d = true
  e = 1

  list = [
    hv.b
  ]

  nestedmap = {
    submap = {
      subsubmap = {
        hello = hv.a
        world = true
      }
    }
  }
}
