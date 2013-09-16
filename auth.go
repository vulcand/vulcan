package vulcan

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
)

type AuthRequest struct {
	Username string
	Password string
	Protocol string
	Method   string
	Url      string
	Length   int64
	Ip       string
	Headers  map[string][]string
}

//Will be used when unmarshalling
//from json
type AuthResponseObj struct {
	Tokens    []TokenObj
	Upstreams []UpstreamObj
	Headers   map[string][]string
}

type AuthResponse struct {
	Tokens    []Token
	Upstreams []Upstream
	Headers   http.Header
}

func NewAuthResponse(tokens []Token, upstreams []Upstream, headers http.Header) (*AuthResponse, error) {
	if len(tokens) <= 0 {
		return nil, fmt.Errorf("At least one token is required")
	}

	if len(upstreams) <= 0 {
		return nil, fmt.Errorf("At least one upstream is required")
	}

	return &AuthResponse{
		Tokens:    tokens,
		Upstreams: upstreams,
		Headers:   headers}, nil
}

func authResponseFromJson(bytes []byte) (*AuthResponse, error) {
	var a AuthResponseObj
	err := json.Unmarshal(bytes, &a)
	if err != nil {
		return nil, err
	}

	upstreams, err := upstreamsFromJsonList(a.Upstreams)
	if err != nil {
		return nil, err
	}

	tokens, err := tokensFromJsonList(a.Tokens)
	if err != nil {
		return nil, err
	}

	return NewAuthResponse(tokens, upstreams, a.Headers)
}

func FromHttpRequest(r *http.Request) (*AuthRequest, error) {
	auth, err := ParseAuthHeader(r.Header.Get("Authorization"))
	if err != nil {
		return nil, err
	}

	auth_request := &AuthRequest{
		Username: auth.Username,
		Password: auth.Password,
		Protocol: r.Proto,
		Method:   r.Method,
		Url:      r.RequestURI,
		Length:   r.ContentLength,
		Headers:  r.Header,
	}

	for k, v := range r.Header {
		auth_request.Headers[k] = v
	}
	return auth_request, nil
}

func (r *AuthRequest) authQuery(authServer *url.URL) (*url.URL, error) {
	u := CopyUrl(authServer)

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

func (r *AuthRequest) authorize(authServer *url.URL) (*AuthResponse, *HttpError) {
	authUrl, err := r.authQuery(authServer)
	if err != nil {
		LogError(
			"Failed to create query for authServer %s, err %s",
			authServer, err)
		return nil, NewInternalError()
	}

	response, err := http.Get(authUrl.String())
	if err != nil {
		LogError(
			"Failed to execute auth request to server %s, err %s",
			authServer, err)
		return nil, NewInternalError()
	}

	defer response.Body.Close()
	responseBody, err := ioutil.ReadAll(response.Body)
	if err != nil {
		LogError(
			"Failed to read response from auth server %s error: %s",
			authServer, err)
		return nil, NewInternalError()
	}

	LogMessage("AuthServer replies: %s", responseBody)

	if response.StatusCode >= 300 || response.StatusCode < 200 {
		LogMessage(
			"Auth server %s rejected request %s with response %s",
			authServer, r, response.StatusCode)

		return nil, &HttpError{
			StatusCode: response.StatusCode,
			Status:     response.Status,
			Body:       responseBody}
	}

	LogMessage("AuthServer granted request")

	authResponse, err := authResponseFromJson(responseBody)
	if err != nil {
		LogError(
			"Failed to decode auth response %s error: %s",
			responseBody, err)
		return nil, NewInternalError()
	}

	return authResponse, nil
}
