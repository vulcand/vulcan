package js

import (
	"bytes"
	"encoding/json"
	"net/http"
)

type ResponseWriter struct {
	Headers http.Header
	Code    int
	Bytes   *bytes.Buffer
}

func NewResponseWriter() *ResponseWriter {
	return &ResponseWriter{
		Headers: make(http.Header),
		Bytes:   &bytes.Buffer{},
	}
}

func (w *ResponseWriter) Header() http.Header {
	return w.Headers
}

func (w *ResponseWriter) WriteHeader(code int) {
	w.Code = code
}

func (w *ResponseWriter) Write(b []byte) (int, error) {
	return w.Bytes.Write(b)
}

func (w *ResponseWriter) ToReply() map[string]interface{} {
	reply := make(map[string]interface{})
	reply["code"] = w.Code
	bytes := w.Bytes.Bytes()
	var out interface{}
	err := json.Unmarshal(bytes, &out)
	if err != nil {
		reply["message"] = out
	} else {
		reply["message"] = string(bytes)
	}
	reply["headers"] = w.Headers
	return reply
}
