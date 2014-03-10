package errors

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type HttpError interface {
	GetStatusCode() int
	GetBody() []byte
	GetContentType() string
	Error() string
}

type Formatter interface {
	FromStatus(int) HttpError
}

type JsonFormatter struct {
}

func (f *JsonFormatter) FromStatus(statusCode int) HttpError {
	encodedError, err := json.Marshal(map[string]interface{}{
		"error": http.StatusText(statusCode),
	})

	if err != nil {
		panic(err)
	}

	return &HttpJsonError{
		StatusCode: statusCode,
		Body:       encodedError,
	}
}

type HttpJsonError struct {
	StatusCode int
	Body       []byte
}

func (r *HttpJsonError) Error() string {
	return fmt.Sprintf(
		"HttpError(code=%d, body=%s)", r.StatusCode, r.Body)
}

func (r *HttpJsonError) GetStatusCode() int {
	return r.StatusCode
}

func (r *HttpJsonError) GetBody() []byte {
	return r.Body
}

func (r *HttpJsonError) GetContentType() string {
	return "application/json"
}
