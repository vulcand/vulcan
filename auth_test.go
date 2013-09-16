package vulcan

import (
	"net/http"
	"testing"
)

func TestFromHttpSuccess(t *testing.T) {
	requests := []struct {
		In  http.Request
		Out AuthRequest
	}{
		{
			http.Request{
				Method: "GET",
				Header: map[string][]string{
					"Authorization": []string{"Basic QWxhZGRpbjpvcGVuIHNlc2FtZQ=="},
				}},
			AuthRequest{Method: "GET"},
		},
	}
	for _, r := range requests {
		_, err := FromHttpRequest(&r.In)
		if err != nil {
			t.Fatalf("Unexpected error for request [%s], [%s]", r.In, err)
		}
	}
}

func TestFromHttpFail(t *testing.T) {
	requests := []http.Request{
		http.Request{
			Method: "GET",
			Header: map[string][]string{
				"Authorization": []string{"Broken auth"},
			}},
	}
	for _, r := range requests {
		_, err := FromHttpRequest(&r)
		if err == nil {
			t.Fatalf("Expected error for request [%s]", r)
		}
	}
}
