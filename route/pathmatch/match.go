/*Defines router that matches incoming requests by matching incoming paths to regular expressions */
package pathmatch

import (
	"bytes"
	"fmt"
	. "github.com/mailgun/vulcan/location"
	. "github.com/mailgun/vulcan/request"
	"regexp"
	"sort"
	"sync"
)

// Matches the location by path regular expression.
// Out of two paths will select the one with the longer regular expression
type PathMatcher struct {
	locations  []locPair
	expression *regexp.Regexp
	mutex      *sync.Mutex
}

type locPair struct {
	pattern  string
	location Location
}

type ByPattern []locPair

func (a ByPattern) Len() int           { return len(a) }
func (a ByPattern) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByPattern) Less(i, j int) bool { return len(a[i].pattern) > len(a[j].pattern) }

func NewPathMatcher() *PathMatcher {
	return &PathMatcher{
		mutex: &sync.Mutex{},
	}
}

func (m *PathMatcher) Route(req Request) (Location, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.expression == nil {
		return nil, nil
	}

	path := req.GetHttpRequest().URL.Path
	if len(path) == 0 {
		path = "/"
	}
	matches := m.expression.FindStringSubmatchIndex(path)
	if len(matches) < 2 {
		return nil, nil
	}
	for i := 2; i < len(matches); i += 2 {
		if matches[i] != -1 {
			if i/2-1 >= len(m.locations) {
				return nil, fmt.Errorf("Internal logic error: %d", i/2-1)
			}
			return m.locations[i/2-1].location, nil
		}
	}

	return nil, nil
}

func (m *PathMatcher) AddLocation(pattern string, location Location) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	_, err := regexp.Compile(pattern)
	if err != nil {
		return fmt.Errorf("Pattern '%s' does not compile into regular expression: %s", pattern, err)
	}

	for _, p := range m.locations {
		if p.pattern == pattern {
			return fmt.Errorf("Pattern: %s already exists")
		}
	}

	locations := append(m.locations, locPair{pattern, location})

	sort.Sort(ByPattern(locations))
	expression, err := buildMapping(locations)
	if err != nil {
		return err
	}

	m.locations = locations
	m.expression = expression

	return nil
}

func (m *PathMatcher) RemoveLocation(pattern string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	for i, p := range m.locations {
		if p.pattern == pattern {
			// Note this is safe due to the way go does range iterations by snapshotting the ranged list
			m.locations = append(m.locations[:i], m.locations[i+1:]...)
			break
		}
	}
	if len(m.locations) != 0 {
		sort.Sort(ByPattern(m.locations))
	}

	expression, err := buildMapping(m.locations)
	if err == nil {
		m.expression = expression
	} else {
		m.expression = nil
	}
	return err
}

func buildMapping(locations []locPair) (*regexp.Regexp, error) {
	if len(locations) == 0 {
		return nil, fmt.Errorf("No locations")
	}
	out := &bytes.Buffer{}
	out.WriteString("^")
	for i, p := range locations {
		out.WriteString("(")
		out.WriteString(p.pattern)
		out.WriteString(")")
		if i != len(locations)-1 {
			out.WriteString("|")
		}
	}
	// Add optional trailing slash here
	out.WriteString("/?$")
	return regexp.Compile(out.String())
}
