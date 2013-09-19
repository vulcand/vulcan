package vulcan

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
)

// Control request is issued by proxy
// to a control server asking what do do with the request
// Control server replies with structured reply - ProxyInstructions
// or denies request based on it's internal logic
type ControlRequest struct {
	Username string
	Password string
	Protocol string
	Method   string
	Url      string
	Length   int64
	Ip       string
	Headers  map[string][]string
}

// Issues a request to an routing server. Three cases are available after the execution:
// * Request failed. In this case general error is returned.
// * Request has been denied by auth server, in this case HttpError has been returned
// * Requst has been granted and auth server replied with instructions
func getInstructions(controlServer *url.URL, req *http.Request) (*ProxyInstructions, *HttpError, error) {
	r, err := controlRequestFromHttp(req)
	if err != nil {
		return nil, nil, err
	}

	query, err := r.controlQuery(controlServer)
	if err != nil {
		return nil, nil, fmt.Errorf(
			"Failed to create query for controlServer %s, err %s",
			controlServer, err)
	}

	response, err := http.Get(query.String())
	if err != nil {
		return nil, nil, fmt.Errorf(
			"Failed to execute auth request to server %s, err %s",
			controlServer, err)
	}

	defer response.Body.Close()
	responseBody, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, nil, fmt.Errorf(
			"Failed to read response from auth server %s error: %s",
			controlServer, err)
	}

	LogMessage("ControlServer replies: %s", responseBody)

	// Control server denied the request, stream this request
	if response.StatusCode >= 300 || response.StatusCode < 200 {
		return nil, nil, &HttpError{
			StatusCode: response.StatusCode,
			Status:     response.Status,
			Body:       responseBody}
	}

	LogMessage("Control server granted request")

	instructions, err := proxyInstructionsFromJson(responseBody)
	if err != nil {
		return nil, nil, fmt.Errorf(
			"Failed to decode auth response %s error: %s",
			responseBody, err)
	}

	return instructions, nil, nil
}

func controlRequestFromHttp(r *http.Request) (*ControlRequest, error) {
	auth, err := parseAuthHeader(r.Header.Get("Authorization"))
	if err != nil {
		return nil, err
	}

	request := &ControlRequest{
		Username: auth.Username,
		Password: auth.Password,
		Protocol: r.Proto,
		Method:   r.Method,
		Url:      r.RequestURI,
		Length:   r.ContentLength,
		Headers:  r.Header,
	}

	return request, nil
}

func (r *ControlRequest) controlQuery(controlServer *url.URL) (*url.URL, error) {
	u := copyUrl(controlServer)

	encodedHeaders, err := json.Marshal(r.Headers)
	if err != nil {
		return nil, err
	}

	parameters := url.Values{}
	parameters.Add("username", r.Username)
	parameters.Add("password", r.Password)
	parameters.Add("protocol", r.Protocol)
	parameters.Add("method", r.Protocol)
	parameters.Add("url", r.Url)
	parameters.Add("length", fmt.Sprintf("%s", r.Length))
	parameters.Add("headers", string(encodedHeaders))

	u.RawQuery = parameters.Encode()

	return u, nil
}
