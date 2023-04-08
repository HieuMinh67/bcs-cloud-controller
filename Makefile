

DBG_MAKEFILE ?=
ifeq ($(DBG_MAKEFILE),1)
    $(warning ***** starting Makefile for goal(s) "$(MAKECMDGOALS)")
    $(warning ***** $(shell date))
else
    # If we're not debugging the Makefile, don't echo recipes.
    MAKEFLAGS += -s
endif


# Old-skool build tools.
#
# Commonly used targets (see each target for more information):
#   all: Build code.
#   test: Run tests.
#   clean: Clean up.

# It's necessary to set this because some environments don't link sh -> bash.
SHELL := /usr/bin/env bash -o errexit -o pipefail -o nounset
BASH_ENV := ./hack/lib/logging.sh

# Define variables so `make --warn-undefined-variables` works.
PRINT_HELP ?=

# Constants used throughout.
.EXPORT_ALL_VARIABLES:


define CHECK_TEST_HELP_INFO
# Build and run tests.
#
# Args:
#   WHAT: Directory names to test.  All *_test.go files under these
#     directories will be run.  If not specified, "everything" will be tested.
#   TESTS: Same as WHAT.
#   KUBE_COVER: Whether to run tests with code coverage. Set to 'y' to enable coverage collection.
#   GOFLAGS: Extra flags to pass to 'go' when building.
#   GOLDFLAGS: Extra linking flags to pass to 'go' when building.
#   GOGCFLAGS: Additional go compile flags passed to 'go' when building.
#
# Example:
#   make check
#   make test
#   make test WHAT="-k -s build_ami"
#	make test WHAT="-k -s build_ami -s create_instance"
endef
.PHONY: check test
ifeq ($(PRINT_HELP),y)
check test:
	echo "$$CHECK_TEST_HELP_INFO"
else
check test:
	hack/make-rules/test.sh $(WHAT) $(TESTS)
endif

build:
	hack/make-rules/build.sh
