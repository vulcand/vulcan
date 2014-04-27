package roundrobin

import (
	"fmt"
	log "github.com/mailgun/gotools-log"
	timetools "github.com/mailgun/gotools-time"
	"math"
	"time"
)

type FSMState int

const (
	FSMStart    = iota // initial state of the fsm
	FSMProbing  = iota // fsm is trying some theory
	FSMRollback = iota // fsm is rolling back
	FSMRevert   = iota // fsm is getting back to the original state
)

const (
	FSMMaxWeight            = 16384
	FSMGrowFactor           = 8
	FSMDefaultProbingPeriod = 4 * time.Second
)

// This is the tiny DFA that tries to play with weights to improve over the overall error rate
// to see if it helps and falls back if taking the load off the bad upstream makes the situation worse.
type FSMHandler struct {
	timeProvider        timetools.TimeProvider
	backoffDuration     time.Duration      // duration that we use to backoff or apply any theory
	state               FSMState           // Current state of the state machione
	timer               time.Time          // Timer is set to give probing some time to take place
	probedGoodEndpoints []*changedEndpoint // Probing changes endpoint weights and remembers the weight so it can go back in case of failure
	weightChanges       int
}

type changedEndpoint struct {
	failRatioBefore float64
	endpoint        *WeightedEndpoint
	weightBefore    int
}

func NewFSMHandler() (*FSMHandler, error) {
	return NewFSMHandlerWithOptions(&timetools.RealTime{}, FSMDefaultProbingPeriod)
}

func NewFSMHandlerWithOptions(timeProvider timetools.TimeProvider, duration time.Duration) (*FSMHandler, error) {
	if timeProvider == nil {
		return nil, fmt.Errorf("time provider can not be nil")
	}
	if duration < time.Second {
		return nil, fmt.Errorf("supply some backoff duration >= time.Second")
	}
	return &FSMHandler{
		timeProvider:    timeProvider,
		backoffDuration: duration,
	}, nil
}

func (fsm *FSMHandler) reset() {
	fsm.state = FSMStart
	fsm.timer = fsm.timeProvider.UtcNow().Add(-1 * time.Second)
	fsm.probedGoodEndpoints = nil
	fsm.weightChanges = 0
}

func (fsm *FSMHandler) String() string {
	if fsm.timerExpired() {
		return fmt.Sprintf("FSM(state=%s)", stateToString(fsm.state))
	} else {
		return fmt.Sprintf("FSM(state=%s, timer=%s)", stateToString(fsm.state), fsm.timer.Sub(fsm.timeProvider.UtcNow()))
	}
}

func (fsm *FSMHandler) updateWeights(endpoints []*WeightedEndpoint) error {
	if len(endpoints) == 0 {
		fmt.Errorf("No endpoints supplied")
	}
	switch fsm.state {
	case FSMStart:
		return fsm.onStart(endpoints)
	case FSMProbing:
		return fsm.onProbing(endpoints)
	case FSMRollback:
		return fsm.onRollback(endpoints)
	case FSMRevert:
		return fsm.onRollback(endpoints)
	}
	return fmt.Errorf("Invalid state I am in")
}

func (fsm *FSMHandler) onStart(endpoints []*WeightedEndpoint) error {
	failRate := avgFailRate(endpoints)
	// No errors, so let's see if we can recover weights of previosly changed endpoints to the original state
	if failRate == 0 {
		// If we have previoulsy changed endpoints try to restore weights to the original state
		for _, e := range endpoints {
			if e.effectiveWeight != e.weight {
				// Adjust effective weight back to the original weight in stages
				e.setEffectiveWeight(decrease(e.GetOriginalWeight(), e.GetEffectiveWeight()))
				log.Infof("%s RESTORING %s", fsm, e)
				fsm.setTimer()
				fsm.state = FSMRevert
			}
		}
		return nil
	} else {
		log.Infof("%s reports average fail rate %f", fsm, failRate)
		if !metricsReady(endpoints) {
			log.Infof("%s skip cycle, metrics are not ready yet", fsm)
			return nil
		}
		// Select endpoints with highest error rates and lower their weight
		good, bad := splitEndpoints(endpoints)
		log.Infof("%s better endpoints: %s", fsm, good)
		log.Infof("%s worse endpoints: %s", fsm, bad)
		// No endpoints that are different by their quality
		if len(bad) == 0 || len(good) == 0 {
			log.Infof("%s all endpoints behave in the same manner, can do nothing", fsm)
			return nil
		}
		fsm.probedGoodEndpoints = adjustWeights(good, bad)
		fsm.setTimer()
		fsm.state = FSMProbing
		return nil
	}
}

func (fsm *FSMHandler) onProbing(endpoints []*WeightedEndpoint) error {
	if !fsm.timerExpired() {
		return nil
	}
	// Now revise the good endpoints and see if we made situation worse
	for _, e := range fsm.probedGoodEndpoints {
		if e.failRatioBefore > e.endpoint.failRateMeter.GetRate() {
			// Oops, we made it worse, revert the weights back and go to rollback state
			for _, e := range fsm.probedGoodEndpoints {
				e.endpoint.setEffectiveWeight(e.weightBefore)
			}
			fsm.probedGoodEndpoints = nil
			fsm.state = FSMRollback
			fsm.setTimer()
			return nil
		}
	}
	log.Infof("%s probing successfull, COMMITING the new rates", fsm)
	// We have not made the situation worse, so
	// go back to the starting point and continue the cycle
	fsm.state = FSMStart
	return nil
}

func (fsm *FSMHandler) onRollback(endpoints []*WeightedEndpoint) error {
	if !fsm.timerExpired() {
		return nil
	}
	log.Infof("%s Timer expired pals", fsm)
	fsm.state = FSMStart
	return nil
}

func (fsm *FSMHandler) setTimer() {
	fsm.timer = fsm.timeProvider.UtcNow().Add(fsm.backoffDuration)
}

func (fsm *FSMHandler) timerExpired() bool {
	return fsm.timer.Before(fsm.timeProvider.UtcNow())
}

// Splits endpoint into two groups of endpoints with bad performance and good performance. It does compare relative
// performances of the endpoints though, so if all endpoints have the same performance,
func splitEndpoints(endpoints []*WeightedEndpoint) (good []*WeightedEndpoint, bad []*WeightedEndpoint) {
	avg := avgFailRate(endpoints)
	for _, e := range endpoints {
		if greater(e.failRateMeter.GetRate(), avg) {
			bad = append(bad, e)
		} else {
			good = append(good, e)
		}
	}
	return good, bad
}

func metricsReady(endpoints []*WeightedEndpoint) bool {
	for _, e := range endpoints {
		if !e.failRateMeter.IsReady() {
			return false
		}
	}
	return true
}

func adjustWeights(good, bad []*WeightedEndpoint) []*changedEndpoint {
	changedEndpoints := make([]*changedEndpoint, len(good))
	for i, e := range good {
		changed := &changedEndpoint{
			weightBefore:    e.GetEffectiveWeight(),
			failRatioBefore: e.failRateMeter.GetRate(),
			endpoint:        e,
		}
		changedEndpoints[i] = changed
		if increase(e.GetEffectiveWeight()) < FSMMaxWeight {
			e.setEffectiveWeight(increase(e.GetEffectiveWeight()))
			log.Infof("FSM updated weight %s", e)
		}
	}
	return changedEndpoints
}

// Compare two fail rates by neglecting the insignificant differences
func greater(a, b float64) bool {
	return math.Floor(a*10) > math.Ceil(b*10)
}

func avgFailRate(endpoints []*WeightedEndpoint) float64 {
	r := float64(0)
	for _, e := range endpoints {
		eRate := e.failRateMeter.GetRate()
		r += eRate
	}
	return r / float64(len(endpoints))
}

func increase(weight int) int {
	return weight * FSMGrowFactor
}

func decrease(target, current int) int {
	adjusted := current / FSMGrowFactor
	if adjusted < target {
		return target
	} else {
		return adjusted
	}
}

func abs(a int) int {
	if a > 0 {
		return a
	}
	return -1 * a
}

func stateToString(state FSMState) string {
	switch state {
	case FSMStart:
		return "START"
	case FSMProbing:
		return "PROBING"
	case FSMRollback:
		return "ROLLBACK"
	case FSMRevert:
		return "REVERT"
	}
	return "UNKNOWN"
}
