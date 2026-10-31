# Builds the production images.
#   docker buildx bake --load                          local build, tagged :dev
#   VERSION=1.2.3 docker buildx bake --push            publish (after `docker login`), see release.sh
variable "NAMESPACE" {
  default = "glide"
}

# A x.y.z version also gets the x.y and latest tags, and x from 1.0.0 on (a floating 0 would hide breaking changes).
variable "VERSION" {
  default = "dev"
}

variable "REVISION" {
  default = ""
}

function "tags" {
  params = [image]
  result = can(regex("^[0-9]+\\.[0-9]+\\.[0-9]+$", VERSION)) ? concat(
    [
      "${NAMESPACE}/${image}:${VERSION}",
      "${NAMESPACE}/${image}:${join(".", slice(split(".", VERSION), 0, 2))}",
      "${NAMESPACE}/${image}:latest",
    ],
    split(".", VERSION)[0] == "0" ? [] : ["${NAMESPACE}/${image}:${split(".", VERSION)[0]}"],
  ) : ["${NAMESPACE}/${image}:${VERSION}"]
}

group "default" {
  targets = ["api", "web"]
}

target "common" {
  platforms = ["linux/amd64"]
  attest = ["type=provenance,mode=max", "type=sbom"]
  labels = {
    "org.opencontainers.image.source"   = "https://github.com/Glideh/plys"
    "org.opencontainers.image.licenses" = "MIT"
    "org.opencontainers.image.version"  = VERSION
    "org.opencontainers.image.revision" = REVISION
  }
}

target "api" {
  inherits = ["common"]
  context  = "./api"
  target   = "prod"
  args     = { VERSION = VERSION }
  tags     = tags("plys-api")
  labels = {
    "org.opencontainers.image.title"       = "Plys API"
    "org.opencontainers.image.description" = "Plys media library server: API, scanner and transcoder (ffmpeg)."
  }
}

target "web" {
  inherits = ["common"]
  context  = "./web"
  target   = "prod"
  tags     = tags("plys-web")
  labels = {
    "org.opencontainers.image.title"       = "Plys web"
    "org.opencontainers.image.description" = "Plys web interface (nginx), proxies /api to plys-api."
  }
}
