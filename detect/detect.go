/*
Package implements various strategies for detecting endpoints
*/
package detect

import (
	. "github.com/mailgun/vulcan/endpoint"
)

type Detector interface {
	AddEndpoints(...Endpoint)    // Adds active endpoints to the interface
	RemoveEndpoints(...Endpoint) // Removes active endpoints to the interface
	IsDown(Endpoint) bool
}

// Considers endpoint down for certain duration if amount of errors
// over period of time exceeds threshold
type MaxFails struct {
}

type failStats struct {
	count    int
	endpoint Endpoint
}
