test: clean
	go test -v ./... -cover

deps:
	go get -v -u launchpad.net/gocheck

clean:
	find . -name flymake_* -delete

sloccount:
	 find . -name "*.go" -print0 | xargs -0 wc -l
