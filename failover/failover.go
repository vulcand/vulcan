// Package failover contains predicates that define when request should be retried.
package failover

/*
Examples:

* RequestMethodEq("GET") - allows to failover only get requests
* IsNetworkError - allows to failover on errors
* RequestMethodEq("GET") && AttemptsLe(2) && (IsNetworkError || ResponseCodeEq(408))
  This predicate allows failover for GET requests with maximum 2 attempts with failover
  triggered on network errors or when upstream returns special http response code 408.
*/

import (
	"fmt"

	"github.com/mailgun/vulcan/request"
)

// Predicate that defines what request can fail over in case of error or http response
type Predicate func(request.Request) bool

func RequestMethod() requestToString {
	return func(r request.Request) string {
		return r.GetHttpRequest().Method
	}
}

func Attempts() requestToInt {
	return func(r request.Request) int {
		return len(r.GetAttempts())
	}
}

func ResponseCode() requestToInt {
	return func(r request.Request) int {
		attempts := len(r.GetAttempts())
		if attempts == 0 {
			return 0
		}
		lastResponse := r.GetAttempts()[attempts-1].GetResponse()
		if lastResponse == nil {
			return 0
		}
		return lastResponse.StatusCode
	}
}

func IsNetworkError() Predicate {
	return func(r request.Request) bool {
		attempts := len(r.GetAttempts())
		return attempts != 0 && r.GetAttempts()[attempts-1].GetError() != nil
	}
}

func And(fns ...Predicate) Predicate {
	return func(req request.Request) bool {
		for _, fn := range fns {
			if !fn(req) {
				return false
			}
		}
		return true
	}
}

// Function that returns predicate by joining the passed predicates with OR
func Or(fns ...Predicate) Predicate {
	return func(req request.Request) bool {
		for _, fn := range fns {
			if fn(req) {
				return true
			}
		}
		return false
	}
}

func NotP(p Predicate) Predicate {
	return func(r request.Request) bool {
		return !p(r)
	}
}

func Eq(m interface{}, value interface{}) (Predicate, error) {
	switch mapper := m.(type) {
	case requestToString:
		return stringEq(mapper, value)
	case requestToInt:
		return intEq(mapper, value)
	}
	return nil, fmt.Errorf("unsupported argument: %T", m)
}

func Neq(m interface{}, value interface{}) (Predicate, error) {
	p, err := Eq(m, value)
	if err != nil {
		return nil, err
	}
	return NotP(p), nil
}

func Lt(m interface{}, value interface{}) (Predicate, error) {
	switch mapper := m.(type) {
	case requestToInt:
		return intLt(mapper, value)
	}
	return nil, fmt.Errorf("unsupported argument: %T", m)
}

func Gt(m interface{}, value interface{}) (Predicate, error) {
	switch mapper := m.(type) {
	case requestToInt:
		return intGt(mapper, value)
	}
	return nil, fmt.Errorf("unsupported argument: %T", m)
}

func Le(m interface{}, value interface{}) (Predicate, error) {
	switch mapper := m.(type) {
	case requestToInt:
		return intLe(mapper, value)
	}
	return nil, fmt.Errorf("unsupported argument: %T", m)
}

func Ge(m interface{}, value interface{}) (Predicate, error) {
	switch mapper := m.(type) {
	case requestToInt:
		return intGe(mapper, value)
	}
	return nil, fmt.Errorf("unsupported argument: %T", m)
}

func stringEq(m requestToString, val interface{}) (Predicate, error) {
	value, ok := val.(string)
	if !ok {
		return nil, fmt.Errorf("expected string, got %T", val)
	}
	return func(req request.Request) bool {
		return m(req) == value
	}, nil
}

func intEq(m requestToInt, val interface{}) (Predicate, error) {
	value, ok := val.(int)
	if !ok {
		return nil, fmt.Errorf("expected int, got %T", val)
	}
	return func(req request.Request) bool {
		return m(req) == value
	}, nil
}

func intLt(m requestToInt, val interface{}) (Predicate, error) {
	value, ok := val.(int)
	if !ok {
		return nil, fmt.Errorf("expected int, got %T", val)
	}
	return func(req request.Request) bool {
		return m(req) < value
	}, nil
}

func intGt(m requestToInt, val interface{}) (Predicate, error) {
	value, ok := val.(int)
	if !ok {
		return nil, fmt.Errorf("expected int, got %T", val)
	}
	return func(req request.Request) bool {
		return m(req) > value
	}, nil
}

func intLe(m requestToInt, val interface{}) (Predicate, error) {
	value, ok := val.(int)
	if !ok {
		return nil, fmt.Errorf("expected int, got %T", val)
	}
	return func(req request.Request) bool {
		return m(req) <= value
	}, nil
}

func intGe(m requestToInt, val interface{}) (Predicate, error) {
	value, ok := val.(int)
	if !ok {
		return nil, fmt.Errorf("expected int, got %T", val)
	}
	return func(req request.Request) bool {
		return m(req) >= value
	}, nil
}

type requestToString func(req request.Request) string
type requestToInt func(req request.Request) int
