[![Build Status](https://travis-ci.org/mailgun/vulcan.png)](https://travis-ci.org/mailgun/vulcan)
[![Build Status](https://drone.io/github.com/mailgun/vulcan/status.png)](https://drone.io/github.com/mailgun/vulcan/latest)
[![Coverage Status](https://coveralls.io/repos/mailgun/vulcan/badge.png?branch=master)](https://coveralls.io/r/mailgun/vulcan?branch=master)

Status
=======
Don't use it in production, early adopters and hackers are welcome


Reverse proxy library
----------------------

```golang
lb := NewRoundRobin(&MatchAll{Group: "group1"})
lb.AddUpstreams("group1", s.newUpstream(upstream.URL))
proxy := s.newProxy(lb)
```

