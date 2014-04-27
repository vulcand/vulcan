package roundrobin

import (
	"fmt"
	log "github.com/mailgun/gotools-log"
	timetools "github.com/mailgun/gotools-time"
	. "github.com/mailgun/vulcan/endpoint"
	. "github.com/mailgun/vulcan/metrics"
	. "github.com/mailgun/vulcan/request"
	"net/http"
	"net/url"
	"sync"
	"time"
)

type RoundRobin struct {
	mutex         *sync.Mutex
	index         int
	endpoints     []*WeightedEndpoint
	currentWeight int
	weightChanges int
	options       Options
}

type Options struct {
	TimeProvider   timetools.TimeProvider
	FailureHandler FailureHandler // Algorithm that reacts on the failures, adjusting weights

}

func NewRoundRobin() (*RoundRobin, error) {
	return NewRoundRobinWithOptions(Options{})
}

func NewRoundRobinWithOptions(o Options) (*RoundRobin, error) {
	o, err := validateOptions(o)
	if err != nil {
		return nil, err
	}
	rr := &RoundRobin{
		options:   o,
		index:     -1,
		mutex:     &sync.Mutex{},
		endpoints: []*WeightedEndpoint{},
	}
	return rr, nil
}

func validateOptions(o Options) (Options, error) {
	if o.TimeProvider == nil {
		o.TimeProvider = &timetools.RealTime{}
	}

	if o.FailureHandler == nil {
		failureHandler, err := NewFSMHandler()
		if err != nil {
			return o, err
		}
		o.FailureHandler = failureHandler
	}
	return o, nil
}

func (r *RoundRobin) NextEndpoint(req Request) (Endpoint, error) {
	e, err := r.nextEndpoint(req)
	if err != nil {
		return e, err
	}
	// This is the first try
	lastAttempt := req.GetLastAttempt()
	if lastAttempt == nil {
		return e, err
	}
	// Try to prevent failover to the same endpoint that we've seen before
	if lastAttempt.GetEndpoint().GetId() == e.GetId() {
		log.Infof("Preventing failover to the same endpoint, whoa")
		return r.nextEndpoint(req)
	}
	return e, err
}

func (r *RoundRobin) nextEndpoint(req Request) (Endpoint, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if r.allDisabled() {
		return nil, fmt.Errorf("All endpoints are disabled")
	}

	if r.options.FailureHandler != nil {
		weightChangesBefore := r.weightChanges
		r.options.FailureHandler.updateWeights(r.endpoints)
		if weightChangesBefore != r.weightChanges {
			r.resetIterator()
		}
	}

	// GCD across all enabled endpoints
	gcd := r.weightGcd()
	// Maximum weight across all enabled endpoints
	max := r.maxWeight()

	for {
		r.index = (r.index + 1) % len(r.endpoints)
		if r.index == 0 {
			r.currentWeight = r.currentWeight - gcd
			if r.currentWeight <= 0 {
				r.currentWeight = max
				if r.currentWeight == 0 {
					return nil, fmt.Errorf("All endpoints have 0 weight")
				}
			}
		}
		e := r.endpoints[r.index]
		if e.effectiveWeight >= r.currentWeight {
			return e.endpoint, nil
		}
	}
	// We did full circle and found nothing
	return nil, fmt.Errorf("No available endpoints!")
}

func (r *RoundRobin) resetIterator() {
	r.index = -1
	r.currentWeight = 0
}

func (r *RoundRobin) resetState() {
	r.resetIterator()
	if r.options.FailureHandler != nil {
		r.options.FailureHandler.reset()
	}
}

func (r *RoundRobin) FindEndpoint(endpoint Endpoint) Endpoint {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	e, _ := r.findEndpointById(endpoint.GetId())
	return e
}

func (r *RoundRobin) FindEndpointById(endpointId string) Endpoint {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	e, _ := r.findEndpointById(endpointId)
	return e
}

func (r *RoundRobin) findEndpointById(endpointId string) (*WeightedEndpoint, int) {
	if len(r.endpoints) == 0 {
		return nil, -1
	}
	for i, e := range r.endpoints {
		if e.endpoint.GetId() == endpointId {
			return e, i
		}
	}
	return nil, -1
}

func (r *RoundRobin) GetEndpoints() []*WeightedEndpoint {
	return r.endpoints
}

func (rr *RoundRobin) AddEndpoint(endpoint Endpoint) error {
	return rr.AddEndpointWithOptions(endpoint, EndpointOptions{})
}

// In case if endpoint is already present in the load balancer, returns error
func (r *RoundRobin) AddEndpointWithOptions(endpoint Endpoint, options EndpointOptions) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if endpoint == nil {
		return fmt.Errorf("Endpoint can't be nil")
	}

	if e, _ := r.findEndpointById(endpoint.GetId()); e != nil {
		return fmt.Errorf("Endpoint already exists")
	}

	we, err := r.newWeightedEndpoint(endpoint, options)
	if err != nil {
		return err
	}

	r.endpoints = append(r.endpoints, we)
	r.resetState()
	return nil
}

func (rr *RoundRobin) newWeightedEndpoint(endpoint Endpoint, options EndpointOptions) (*WeightedEndpoint, error) {
	// Treat weight 0 as a default value passed by customer
	if options.Weight == 0 {
		options.Weight = 1
	}
	if options.Weight < 0 {
		return nil, fmt.Errorf("Weight should be >=0")
	}

	meter, err := NewFailRateMeter(
		endpoint, 10, time.Second, rr.options.TimeProvider, IsNetworkError)
	if err != nil {
		return nil, err
	}

	return &WeightedEndpoint{
		failRateMeter:   meter,
		endpoint:        endpoint,
		weight:          options.Weight,
		effectiveWeight: options.Weight,
		disabled:        options.Disabled,
		rr:              rr,
	}, nil
}

func (r *RoundRobin) RemoveEndpoint(endpoint Endpoint) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	e, index := r.findEndpointById(endpoint.GetId())
	if e == nil {
		return fmt.Errorf("Endpoint not found")
	}
	r.endpoints = append(r.endpoints[:index], r.endpoints[index+1:]...)
	r.resetState()
	return nil
}

func (rr *RoundRobin) ProcessRequest(Request) (*http.Response, error) {
	return nil, nil
}

func (rr *RoundRobin) ProcessResponse(req Request, a Attempt) {
}

func (rr *RoundRobin) ObserveRequest(Request) {
}

func (rr *RoundRobin) ObserveResponse(req Request, a Attempt) {
	rr.mutex.Lock()
	defer rr.mutex.Unlock()

	// Update stats for the endpoint after the request was done
	endpoint := req.GetLastAttempt().GetEndpoint()
	if endpoint == nil {
		return
	}
	we, _ := rr.findEndpointById(endpoint.GetId())
	if we == nil {
		return
	}
	we.failRateMeter.ObserveResponse(req, a)
}

func (rr *RoundRobin) allDisabled() bool {
	for _, e := range rr.endpoints {
		if !e.isDisabled() {
			return false
		}
	}
	return true
}

func (rr *RoundRobin) maxWeight() int {
	max := -1
	for _, e := range rr.endpoints {
		if e.effectiveWeight > max {
			max = e.effectiveWeight
		}
	}
	return max
}

func (rr *RoundRobin) weightGcd() int {
	divisor := -1
	for _, e := range rr.endpoints {
		if e.isDisabled() {
			continue
		}
		if divisor == -1 {
			divisor = e.effectiveWeight
		} else {
			divisor = gcd(divisor, e.effectiveWeight)
		}
	}
	return divisor
}

func gcd(a, b int) int {
	for b != 0 {
		a, b = b, a%b
	}
	return a
}

// Set additional parameters for the endpoint this load balancer supports
type EndpointOptions struct {
	Weight   int  // Relative weight for the enpoint to other enpoints in the load balancer
	Disabled bool // Whether this endpoint is disabled
}

type WeightedEndpoint struct {
	failRateMeter   *FailRateMeter
	endpoint        Endpoint
	weight          int
	effectiveWeight int
	disabled        bool
	disabledUntil   time.Time
	rr              *RoundRobin
}

func (we *WeightedEndpoint) String() string {
	return fmt.Sprintf("WeightedEndpoint(id=%s, url=%s, weight=%d, effectiveWeight=%d, failRate=%f)",
		we.GetId(), we.GetUrl(), we.weight, we.effectiveWeight, we.failRateMeter.GetRate())
}

func (we *WeightedEndpoint) GetId() string {
	return we.endpoint.GetId()
}

func (we *WeightedEndpoint) GetUrl() *url.URL {
	return we.endpoint.GetUrl()
}

func (we *WeightedEndpoint) isDisabled() bool {
	return we.disabled || we.disabledUntil.After(we.rr.options.TimeProvider.UtcNow())
}

func (we *WeightedEndpoint) setEffectiveWeight(w int) {
	we.rr.weightChanges += 1
	we.effectiveWeight = w
}

func (we *WeightedEndpoint) GetOriginalWeight() int {
	return we.weight
}

func (we *WeightedEndpoint) GetEffectiveWeight() int {
	return we.effectiveWeight
}

func (we *WeightedEndpoint) GetMetrics() *FailRateMeter {
	return we.failRateMeter
}
