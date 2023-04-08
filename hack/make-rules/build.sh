#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

echo "Building MicroK8s image"

hack/packer/build-ubuntu-image.sh