variable "IMAGE" { default = "haozhi152/agent-platform" }
variable "TAG" { default = "local" }
variable "PLATFORMS" { default = [] }
variable "SOURCE_REVISION" { default = "development" }

target "runtime" {
  context = "."
  dockerfile = "deploy/runtimes/unified/Dockerfile"
  platforms = PLATFORMS
}

target "app" {
  context = "."
  dockerfile = "deploy/all-in-one/Dockerfile"
  contexts = { runtime = "target:runtime" }
  platforms = PLATFORMS
  tags = ["${IMAGE}:${TAG}"]
  args = { SOURCE_REVISION = SOURCE_REVISION }
  attest = ["type=provenance,disabled=true"]
}
