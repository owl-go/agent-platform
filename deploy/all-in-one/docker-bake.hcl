variable "IMAGE" { default = "haozhi152/agent-platform" }
variable "TAG" { default = "local" }
variable "RUNTIME_IMAGE" { default = "haozhi152/agent-platform-runtime" }
variable "RUNTIME_TAG" { default = "local" }
variable "PLATFORMS" { default = [] }
variable "SOURCE_REVISION" { default = "development" }

target "runtime" {
  context = "."
  dockerfile = "deploy/runtimes/unified/Dockerfile"
  platforms = PLATFORMS
  tags = ["${RUNTIME_IMAGE}:${RUNTIME_TAG}"]
  attest = ["type=provenance,disabled=true"]
}

target "app" {
  context = "."
  dockerfile = "deploy/all-in-one/Dockerfile"
  platforms = PLATFORMS
  tags = ["${IMAGE}:${TAG}"]
  args = { SOURCE_REVISION = SOURCE_REVISION }
  attest = ["type=provenance,disabled=true"]
}
