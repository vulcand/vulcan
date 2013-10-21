test: clean
	go test -v ./...
logtest:clean
	CASSANDRA=yes go test -v ./... -gocheck.f "LogUtilsSuite.*"
cstest:clean
	CASSANDRA=yes go test -v ./... -gocheck.f "CassandraBackendSuite.*"
pqtest:clean
	go test -v ./loadbalance/roundrobin -gocheck.f "PQSuite.*"
rrtest:clean
	go test -v ./loadbalance/roundrobin -gocheck.f "RoundRobinSuite.*"
rrtest-coverage:clean
	gocov test -v  ./loadbalance/roundrobin -gocheck.f "RoundRobinSuite.*" | gocov report
pqtest-coverage:clean
	gocov test -v ./loadbalance/roundrobin | gocov report
coverage: clean
	gocov test -v ./... | gocov report
annotate: clean
	FILENAME=$(shell uuidgen)
	gocov test  > /tmp/--go-test-server-coverage.json
	gocov annotate /tmp/--go-test-server-coverage.json $(fn)
all:
	go install github.com/mailgun/vulcan # installs library
	go install github.com/mailgun/vulcan/vulcan # and service
clean:
	find -name flymake_* -delete
run: all
	GOMAXPROCS=4 vulcan -stderrthreshold=INFO -logtostderr=true -c=http://localhost:5000 -b=memory -lb=roundrobin -log_dir=/tmp -logcleanup=24h
runcs: all
	GOMAXPROCS=4 vulcan -stderrthreshold=INFO -logtostderr=true -c=http://localhost:6263 -b=cassandra -lb=roundrobin -csnode=localhost -cskeyspace=vulcan_dev -cscleanup=true -cscleanuptime=19:05 -log_dir=/tmp
sloccount:
	 find . -name "*.go" -print0 | xargs -0 wc -l
