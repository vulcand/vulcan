package command

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type Reply struct {
	Code int
	Body interface{}
}

// On every request proxy asks control server what to do
// with the request, control server replies with this structure
// or rejects the request.
type Forward struct {
	// Allows proxy to fall back to the next upstream
	// if the selected upstream failed
	Failover *Failover
	// Tokens uniquely identify the requester. E.g. token can be account id or
	// combination of ip and account id. Tokens can be throttled as well.
	// The reply can have 0 or several tokens
	Rates map[string][]*Rate
	// List of upstreams that can accept this request. Load balancer will
	// choose an upstream based on the algo, e.g. random, round robin,
	// or least connections. At least one upstream is required.
	Upstreams []*Upstream
	// If supplied, headers will be added to the proxied request.
	AddHeaders    http.Header
	RemoveHeaders []string
	RewritePath   string
}

func NewForward(
	failover *Failover,
	rates map[string][]*Rate,
	upstreams []*Upstream,
	addHeaders http.Header,
	removeHeaders []string) (*Forward, error) {

	if len(upstreams) <= 0 {
		return nil, fmt.Errorf("At least one upstream is required")
	}

	return &Forward{
		Failover:      failover,
		Rates:         rates,
		Upstreams:     upstreams,
		AddHeaders:    addHeaders,
		RemoveHeaders: removeHeaders,
	}, nil
}

func NewCommandFromObj(in interface{}) (interface{}, error) {
	obj, ok := in.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("Command: expected dictionary, got %T", in)
	}
	_, exists := obj["code"]
	if exists {
		return NewReplyFromDict(obj)
	} else {
		return NewForwardFromDict(obj)
	}
}

func NewReplyFromDict(in map[string]interface{}) (interface{}, error) {
	codeI, exists := in["code"]
	if !exists {
		return nil, fmt.Errorf("Expected code")
	}
	code := 0
	switch codeC := codeI.(type) {
	case int:
		code = codeC
	case float64:
		if codeC != float64(int(codeC)) {
			return nil, fmt.Errorf("HTTP code should be an integer, got %v", code)
		}
		code = int(codeC)
	default:
		return nil, fmt.Errorf("HTTP code should be an integer, got %v", code)
	}

	bodyI, exists := in["body"]
	if !exists {
		return nil, fmt.Errorf("Expected body")
	}
	_, err := json.Marshal(bodyI)
	if err != nil {
		return nil, fmt.Errorf("Property 'body' should be json encodeable")
	}
	return &Reply{Code: code, Body: bodyI}, nil
}

func NewForwardFromDict(in map[string]interface{}) (interface{}, error) {
	upstreamsI, exists := in["upstreams"]
	if !exists {
		return nil, fmt.Errorf("Upstreams are required")
	}
	var err error
	upstreams, err := NewUpstreamsFromObj(upstreamsI)
	if err != nil {
		return nil, err
	}

	ratesI, exists := in["rates"]
	var rates map[string][]*Rate
	if exists {
		rates, err = NewRatesFromObj(ratesI)
		if err != nil {
			return nil, err
		}
	}

	failoverI, exists := in["failover"]
	var failover *Failover
	if exists {
		failover, err = NewFailoverFromObj(failoverI)
		if err != nil {
			return nil, err
		}
	}

	pathI, exists := in["rewrite_path"]
	ok := false
	rewritePath := ""
	if exists {
		rewritePath, ok = pathI.(string)
		if !ok {
			return nil, fmt.Errorf("Rewrite-path should be a string")
		}
	}

	addHeaders, removeHeaders, err := AddRemoveHeadersFromDict(in)
	if err != nil {
		return nil, err
	}

	return &Forward{
		Rates:         rates,
		Failover:      failover,
		Upstreams:     upstreams,
		AddHeaders:    addHeaders,
		RemoveHeaders: removeHeaders,
		RewritePath:   rewritePath,
	}, nil
}
