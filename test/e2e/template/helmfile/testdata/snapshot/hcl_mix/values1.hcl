locals {
  filename = "values1.hcl"
}

values {
  a = "a"
  b = "b"
  c = "${hv.a}${hv.b}"

  list = [
    hv.b
  ]

  locals = local

  // Maps and some functions
  nestedmap = {
    submap = {
      subsubmap = {
        hello = hv.a
      }
    }
  }

  nestedmap2 = {
    submap = {
      subsubmap = {
        hello = hv.c
      }
    }
  }

  merged = merge(hv.nestedmap, hv.nestedmap2)
  fromMap = "${hv.merged.submap.subsubmap.hello}"

  // precedence demo
  yamlOverride = "yaml_overrode"


  // Simple Expressions
  ternary = true ? true : false
  expressionInText = "%{if hv.ternary }yes%{else}no%{endif}"
  insideFor = "%{for i in hv.list }${i}%{endfor}"
  simpleCompute = 2 + 2

  multiBlock = hv.block

  crossfile = hv.crossfile_var
}

values {
  block = "block"
}