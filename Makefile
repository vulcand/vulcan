test:
	go test
coverage:
	gocov test | gocov report
annotate:
	FILENAME=$(shell uuidgen)
	gocov test  > /tmp/--go-test-server-coverage.json
	gocov annotate /tmp/--go-test-server-coverage.json $(fn)
all:
	go install github.com/mailgun/vulcan
	go install github.com/mailgun/vulcan/vulcan
run:all
	GOMAXPROCS=4 vulcan
