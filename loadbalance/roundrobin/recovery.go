package roundrobin

type FailureHandler interface {
	// WeightedRoundRobin sends the endpoints with weights as well as the current error rate
	// so the policy can make some intelligent choices and update weights.
	// returns true in case if any of the endpoints weights have been updated, false otherwise

	// returns error if something bad happened
	updateWeights(endpoints []*weightedEndpoint) error
	// Should provide an ability to reset itself
	reset()
}
