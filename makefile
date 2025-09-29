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
