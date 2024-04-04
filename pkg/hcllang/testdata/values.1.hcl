locals {
  myLocal = "local"
  myLocalRef = local.myLocal
}

values {
  val1 = 1
  val2 = upper(local.myLocal)
  val3 = "${local.myLocal}${hv.val1}"
  val4 = min(hv.val1, 10, -1)
}