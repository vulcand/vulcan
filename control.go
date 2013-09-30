package vulcan

import (
	"encoding/json"
	"fmt"
	"github.com/golang/glog"
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

// Issues a request to an routing server. Three outcomes are possible:
//
// * Request failed. In this case general error is returned.
// * Request has been denied by auth server, in this case HttpError is returned
// * Requst has been granted and auth server replied with instructions
//
func getInstructions(httpClient *http.Client, controlServers []*url.URL, req *http.Request) (*ProxyInstructions, error) {
	controlRequest, err := controlRequestFromHttp(req)
	if err != nil {
		if _, ok := err.(AuthError); ok {
			glog.Errorf("Failed to create control request: %s", err)
			return nil, NewHttpError(http.StatusProxyAuthRequired)
		}
		return nil, err
	}

	for _, controlServer := range controlServers {
		instructions, err := queryServer(httpClient, controlServer, controlRequest)
		if err != nil {
			// This is http error that we'd like to transfer to the client
			_, isHttp := err.(*HttpError)
			if isHttp {
				glog.Errorf("Control server %s denied request %s", err)
				return nil, err
			} else {
				glog.Errorf("Control server %s failed: %s, try another", err)
			}
		} else {
			return instructions, err
		}
	}
	return nil, fmt.Errorf("All control servers failed.")
}

func queryServer(httpClient *http.Client, controlServer *url.URL, controlRequest *ControlRequest) (*ProxyInstructions, error) {

	query, err := controlRequest.controlQuery(controlServer)
	if err != nil {
		return nil, fmt.Errorf(
			"Failed to create query for controlServer %s, err %s",
			controlServer, err)
	}

	response, err := httpClient.Get(query.String())
	if err != nil {
		return nil, fmt.Errorf(
			"Control request failed. Server %s, error: '%s'",
			controlServer, err)
	}

	defer response.Body.Close()
	responseBody, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf(
			"Failed to read response from auth server %s error: %s",
			controlServer, err)
	}
	glog.Infof("ControlServer replies: \n-->%s<--\n", responseBody)

	// Control server denied the request, stream this request
	if response.StatusCode >= 300 || response.StatusCode < 200 {
		return nil, &HttpError{
			StatusCode: response.StatusCode,
			Status:     response.Status,
			Body:       responseBody}
	}

	instructions, err := proxyInstructionsFromJson(responseBody)
	if err != nil {
		return nil, fmt.Errorf(
			"Failed to decode auth response %s error: %s",
			responseBody, err)
	}

	return instructions, nil
}

func controlRequestFromHttp(r *http.Request) (*ControlRequest, error) {
	auth, err := parseAuthHeader(r.Header.Get("Authorization"))
	if err != nil {
		return nil, AuthError(err.Error())
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
	parameters.Add("method", r.Method)
	parameters.Add("url", r.Url)
	parameters.Add("length", fmt.Sprintf("%d", r.Length))
	parameters.Add("headers", string(encodedHeaders))

	u.RawQuery = parameters.Encode()

	return u, nil
}
