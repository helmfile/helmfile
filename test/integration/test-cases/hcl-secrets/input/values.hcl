values {
  // Secrets and precedence demo
  secretref = upper(hv.secret)
  yamlOverride = "yaml_overrode"
  secretOveriddenByPrecedence = "will_be_overwrittten"
}
