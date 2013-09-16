package vulcan

import (
	"testing"
)

//Just to make sure we don't panic, return err and not
//username and pass and cover the function
func TestParseBadHeaders(t *testing.T) {
	headers := []string{
		//just empty string
		"",
		//missing auth type
		"justplainstring",
		//unknown auth type
		"Whut justplainstring",
		//invalid base64
		"Basic Shmasic",
		//random encoded string
		"Basic YW55IGNhcm5hbCBwbGVhcw==",
	}
	for _, h := range headers {
		_, err := ParseAuthHeader(h)
		if err == nil {
			t.Fatalf("Expected error for header [%s]", h)
		}
	}
}

//Just to make sure we don't panic, return err and not
//username and pass and cover the function
func TestParseSuccess(t *testing.T) {
	headers := []struct {
		Header   string
		Expected BasicAuth
	}{
		{
			"Basic QWxhZGRpbjpvcGVuIHNlc2FtZQ==",
			BasicAuth{Username: "Aladdin", Password: "open sesame"},
		},
		//empty pass
		{
			"Basic QWxhZGRpbjo=",
			BasicAuth{Username: "Aladdin", Password: ""},
		},
	}
	for _, h := range headers {
		request, err := ParseAuthHeader(h.Header)
		if err != nil {
			t.Fatalf("Unexpected error for header [%s], [%s]", h, err)
		}

		if request.Username != h.Expected.Username {
			t.Fatalf("Username [%s] does not match expected [%s]",
				request.Username,
				h.Expected.Username)
		}

		if request.Password != h.Expected.Password {
			t.Fatalf("Password [%s] not match expected [%s]",
				request.Password,
				h.Expected.Password)
		}
	}
}

// We should panic with wrong args
func TestRandomRangeFail(t *testing.T) {
	panicked := false
	defer func() {
		if !panicked {
			t.Fatalf("Expected panic")
		}
	}()
	defer func() {
		r := recover()
		if r != nil {
			panicked = true
		}
	}()
	RandomRange(0, 0)
}

// Just make sure we don't panic on good args
func TestRandomSuccess(t *testing.T) {
	RandomRange(0, 1)
	RandomRange(2, 4)
}
