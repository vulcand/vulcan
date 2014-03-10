test: clean
	go test -v ./... -cover

deps:
	go get -v -u launchpad.net/gocheck
	go get -v -u github.com/mailgun/gotools-log

clean:
	find . -name flymake_* -delete

sloccount:
	 find . -name "*.go" -print0 | xargs -0 wc -l
