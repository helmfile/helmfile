locals {
  localInValues7File = "localInValues7"
}

values {
  a = "a"
  b = "b"
  c = "${hv.a}${hv.b}"

  map = {
    "a" = "${hv.a}"
  } 

  list = [
    hv.b
  ]

  nestedmap = {
    submap = {
      subsubmap = {
        hello = hv.c
      }
    }
  }

  ternary = true ? true : false

  fromMap = "${hv.map.a}${hv.nestedmap.submap.subsubmap.hello}"

  expressionInText = "%{if hv.ternary }yes%{else}no%{endif}"
  insideFor = "%{for i in hv.list }${i}%{endfor}"

  multi_block = hv.block

  crossfile = hv.crossfile_var

  localRef = local.localInValues7File
}

values {
  block = "block"
}