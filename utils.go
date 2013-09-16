package vulcan

import (
	"encoding/base64"
	"fmt"
	"math/rand"
	"net/url"
	"strings"
)

type BasicAuth struct {
	Username string
	Password string
}

func ParseAuthHeader(header string) (*BasicAuth, error) {

	values := strings.Fields(header)
	if len(values) != 2 {
		return nil, fmt.Errorf(
			fmt.Sprintf("Failed to parse header '%s'", header))
	}

	auth_type := strings.ToLower(values[0])
	if auth_type != "basic" {
		return nil, fmt.Errorf("Expected basic auth type, got '%s'", auth_type)
	}

	encoded_string := values[1]
	decoded_string, err := base64.StdEncoding.DecodeString(encoded_string)
	if err != nil {
		err = fmt.Errorf(
			"Failed to parse header '%s', base64 failed: %s", header, err)
		return nil, err
	}

	values = strings.SplitN(string(decoded_string), ":", 2)
	if len(values) != 2 {
		err = fmt.Errorf(
			"Failed to parse header '%s', expected separator ':'", header, err)
		return nil, err
	}
	return &BasicAuth{Username: values[0], Password: values[1]}, nil
}

//Returns, as an int, a non-negative pseudo-random number
//in [min,max). It panics if min <= 0.
func RandomRange(min int, max int) int {
	if max-min <= 0 {
		panic(fmt.Sprintf("Invalid range [%d,%d]", min, max))
	}
	return rand.Intn(max-min) + min
}

func CopyUrl(in *url.URL) *url.URL {
	out := new(url.URL)
	*out = *in
	if in.User != nil {
		*out.User = *in.User
	}
	return out
}
