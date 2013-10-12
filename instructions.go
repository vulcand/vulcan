package vulcan

import (
	"net/http"
)

// On every request proxy asks control server what to do
// with the request, control server replies with this structure
// or rejects the request.
type ProxyInstructions struct {
	// Allows proxy to fall back to the next upstream
	// if the selected upstream failed
	Failover bool
	// Tokens uniquely identify the requester. E.g. token can be account id or
	// combination of ip and account id. Tokens can be throttled as well.
	// The reply can have 0 or several tokens
	Tokens []*Token
	// List of upstreams that can accept this request. Load balancer will
	// choose an upstream based on the algo, e.g. random, round robin,
	// or least connections. At least one upstream is required.
	Upstreams []*Upstream
	// If supplied, headers will be added to the proxied request.
	Headers http.Header
}
