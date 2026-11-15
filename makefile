WORK_DIR=cmd/caterpillar

# define project directories to be covered for test cases
directories=

.PHONY: all
all: build test

# check whether project is building successfully :
.PHONY: build
build: 
	go build -o caterpillar ${WORK_DIR}/caterpillar.go

# check test cases / code coverage 
.PHONY: test
test: 
	$(foreach dir,$(directories),go test -v  $(dir) -p 1 -coverprofile=c.out; cat c.out >> overall_coverage.out;)

.PHONY: report
report:
	awk '/mode: set/ && ++f != 1 {getline} 1' overall_coverage.out > coverage_report.out
	go tool cover -html=coverage_report.out -o coverage_report.html

# ---------------------------------------------------------------------------
# Quality gates. CI (.github/workflows/ci.yaml) calls these targets, so they
# run the same way locally and in a pull request.
# ---------------------------------------------------------------------------

# Tool versions. Keep these in step with .pre-commit-config.yaml.
GOLANGCI_LINT_VERSION ?= v2.15.0
GO_LICENSES_VERSION   ?= v2.0.1

# The commit that `make lint` compares against: only issues introduced since
# it are reported. CI sets it to the pull request's base commit.
LINT_BASE ?= origin/main

# Licenses that third-party code linked into caterpillar may use. Zlib and
# BSD-2-Clause-NetBSD cover parts of the librdkafka sources vendored by
# confluent-kafka-go; both are permissive and need only attribution.
ALLOWED_LICENSES ?= Apache-2.0,MIT,BSD-2-Clause,BSD-3-Clause,ISC,MPL-2.0,Zlib,BSD-2-Clause-NetBSD

# Fails if any Go file is not gofmt-formatted, and lists the files.
.PHONY: fmt-check
fmt-check:
	@out="$$(gofmt -l .)"; \
	if [ -n "$$out" ]; then echo "These files are not gofmt-formatted (run gofmt -w):"; echo "$$out"; exit 1; fi

# copylocks is disabled for three existing findings; see .golangci.yml.
.PHONY: vet
vet:
	go vet -copylocks=false ./...

# Every linter in .golangci.yml, reporting only issues new since LINT_BASE.
.PHONY: lint
lint:
	golangci-lint run --new-from-rev=$(LINT_BASE) ./...

# Dependency allow/deny list and vet over the whole tree.
.PHONY: lint-policy
lint-policy:
	golangci-lint run --enable-only=gomodguard_v2,govet ./...

# Fails if any dependency compiled into the binary uses a license outside
# ALLOWED_LICENSES (or one go-licenses cannot identify).
.PHONY: licenses
licenses:
	go-licenses check ./... --allowed_licenses=$(ALLOWED_LICENSES)

# Writes the license and notice files of every third-party dependency to
# third_party_licenses/ (git-ignored), for attaching to a release.
# go-licenses refuses to write into an existing directory.
.PHONY: licenses-save
licenses-save:
	go-licenses save ./... --save_path=third_party_licenses --force

# Installs the tools above at the pinned versions into $(go env GOPATH)/bin.
.PHONY: tools
tools:
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)
	go install github.com/google/go-licenses/v2@$(GO_LICENSES_VERSION)

# Installs the git hooks in .pre-commit-config.yaml (needs pre-commit).
.PHONY: hooks
hooks:
	pre-commit install --install-hooks --hook-type pre-commit
