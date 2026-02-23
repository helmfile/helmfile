locals {
  filename = "values2.hcl"
}

values {
  a = local.filename
  b = "b2"
  c = upper("${hv.a}${hv.b}")
  d = false
  e = 2

  list = [
    hv.a,
    hv.b
  ]

  nestedmap = {
    submap = {
      subsubmap = {
        hello = "hello"
        world = lower("WORLD")
      }
    }
  }
}
