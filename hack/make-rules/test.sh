#!/usr/bin/env bash

# Copyright 2014 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -o errexit
set -o nounset
set -o pipefail

echo "Running E2E Tests"

isnum() {
  [[ "$1" =~ ^[0-9]+$ ]]
}

# https://www.shellscript.sh/examples/getopt/
while getopts "kp:s:" opt; do
  case $opt in
  k)
    #    kube::test::usage
    echo "- Keep AMI after build"
    export SKIP_cleanup_ami=true
    ;;
  s)
    STAGE_TO_SKIP="${OPTARG}"
    export SKIP_${STAGE_TO_SKIP}=true
    echo "- Skipping stage ${STAGE_TO_SKIP}"
    ;;
  :)
    echo "Option -${OPTARG} <value>"
    exit 1
    ;;
  ?) ;;

  esac
done

# https://unix.stackexchange.com/questions/214141/explain-the-shell-command-shift-optind-1
# removes n strings from the positional parameters list. Thus shift $((OPTIND-1))
# removes all the options that have been parsed by getopts from the parameters list,
# and so after that point, $1 will refer to the first non-option argument passed to the script.
shift "$((OPTIND - 1))"

#CMD=${1-default}
#
#case $opt in
#only_build_ami)
#  #    kube::test::usage
#  echo "Only build AMI"
#  #    exit 0
#  ;;
#:)
#  echo "Option -${OPTARG} <value>"
#  exit 1
#  ;;
#?)
#  #    exit 1
#  ;;
#esac

go test -v github.com/beancloudservices/bcs-cloud-controller/test/e2e
