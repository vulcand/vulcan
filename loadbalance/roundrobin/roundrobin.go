package roundrobin

import (
	"fmt"
	timetools "github.com/mailgun/gotools-time"
	. "github.com/mailgun/vulcan/endpoint"
	. "github.com/mailgun/vulcan/metrics"
	. "github.com/mailgun/vulcan/request"
	"net/http"
	"sync"
	"time"
)

type RoundRobin struct {
	failureHandler FailureHandler
	mutex          *sync.Mutex
	index          int
	endpoints      []*weightedEndpoint
	timeProvider   timetools.TimeProvider
	currentWeight  int
	weightChanges  int
}

type RoundRobinOptions struct {
	TimeProvider timetools.TimeProvider
}

func NewRoundRobin() (*RoundRobin, error) {
	failureHandler, err := NewFSMHandler()
	if err != nil {
		return nil, err
	}
	return NewRoundRobinWithOptions(&timetools.RealTime{}, failureHandler)
}

func NewRoundRobinWithOptions(timeProvider timetools.TimeProvider, failureHandler FailureHandler) (*RoundRobin, error) {
	rr := &RoundRobin{
		failureHandler: failureHandler,
		index:          -1,
		mutex:          &sync.Mutex{},
		timeProvider:   timeProvider,
	}
	return rr, nil
}

func (r *RoundRobin) NextEndpoint(req Request) (Endpoint, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if r.allDisabled() {
		return nil, fmt.Errorf("All endpoints are disabled")
	}

	if r.failureHandler != nil {
		weightChangesBefore := r.weightChanges
		r.failureHandler.updateWeights(r.endpoints)
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
	if r.failureHandler != nil {
		r.failureHandler.reset()
	}
}

func (r *RoundRobin) FindEndpoint(endpoint Endpoint) Endpoint {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	e, _ := r.findEndpoint(endpoint)
	return e.endpoint
}

func (r *RoundRobin) findEndpoint(endpoint Endpoint) (*weightedEndpoint, int) {
	if len(r.endpoints) == 0 {
		return nil, -1
	}
	for i, e := range r.endpoints {
		if e.endpoint.GetId() == endpoint.GetId() {
			return e, i
		}
	}
	return nil, -1
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

	if e, _ := r.findEndpoint(endpoint); e != nil {
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

func (rr *RoundRobin) newWeightedEndpoint(endpoint Endpoint, options EndpointOptions) (*weightedEndpoint, error) {
	// Treat weight 0 as a default value passed by customer
	if options.Weight == 0 {
		options.Weight = 1
	}
	if options.Weight < 0 {
		return nil, fmt.Errorf("Weight should be >=0")
	}

	meter, err := NewFailRateMeter(endpoint, 10, time.Second, rr.timeProvider, IsNetworkError)
	if err != nil {
		return nil, err
	}

	return &weightedEndpoint{
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

	e, index := r.findEndpoint(endpoint)
	if e == nil {
		return fmt.Errorf("Endpoint not found")
	}
	r.endpoints = append(r.endpoints[:index], r.endpoints[index+1:]...)
	r.resetState()
	return nil
}

func (rr *RoundRobin) Before(Request) (*http.Response, error) {
	return nil, nil
}

func (rr *RoundRobin) After(req Request) error {
	rr.mutex.Lock()
	defer rr.mutex.Unlock()

	// Update stats for the endpoint after the request was done
	we, _ := rr.findEndpoint(req.GetLastAttempt().GetEndpoint())
	if we == nil {
		return nil
	}
	we.failRateMeter.After(req)
	return nil
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

type weightedEndpoint struct {
	failRateMeter   FailRateGetter
	endpoint        Endpoint
	weight          int
	effectiveWeight int
	disabled        bool
	disabledUntil   time.Time
	rr              *RoundRobin
}

func (we *weightedEndpoint) isDisabled() bool {
	return we.disabled || we.disabledUntil.After(we.rr.timeProvider.UtcNow())
}

func (we *weightedEndpoint) setEffectiveWeight(w int) {
	we.rr.weightChanges += 1
	we.effectiveWeight = w
}

func (we *weightedEndpoint) getOriginalWeight() int {
	return we.weight
}

func (we *weightedEndpoint) getEffectiveWeight() int {
	return we.effectiveWeight
}
