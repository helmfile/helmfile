locals {
  myLocal = "local"
  myLocalRef = local.myLocal
}

values {
  val1 = "1"
  val2 = hv.val1
  val3 = "${local.myLocal}${hv.val1}"
}