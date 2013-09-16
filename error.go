package vulcan

import (
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

func NewInternalError() *HttpError {
	return &HttpError{
		StatusCode: http.StatusInternalServerError,
		Status:     http.StatusText(http.StatusInternalServerError),
		Body:       []byte(http.StatusText(http.StatusInternalServerError))}
}

func NewUpstreamError() *HttpError {
	return &HttpError{
		StatusCode: http.StatusServiceUnavailable,
		Status:     http.StatusText(http.StatusServiceUnavailable),
		Body:       []byte(http.StatusText(http.StatusServiceUnavailable))}
}
