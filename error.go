package vulcan

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type HttpError struct {
	StatusCode int
	Status     string
	Body       []byte
}

func (r *HttpError) Error() string {
	return fmt.Sprintf(
		"HttpError(code=%d, %s, %s)", r.StatusCode, r.Status, r.Body)
}

func NewHttpError(statusCode int) *HttpError {
	return &HttpError{
		StatusCode: statusCode,
		Status:     http.StatusText(statusCode),
		Body:       []byte(http.StatusText(statusCode))}
}

func TooManyRequestsError(retrySeconds int) (*HttpError, error) {

	encodedError, err := json.Marshal(map[string]interface{}{
		"error":         "Too Many Requests",
		"retry-seconds": retrySeconds,
	})

	if err != nil {
		return nil, err
	}

	return &HttpError{
		StatusCode: 429, //RFC 6585
		Status:     "Too Many Requests",
		Body:       encodedError}, nil
}
