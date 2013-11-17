package control

import (
	"net/http"
)

type Controller interface {
	GetInstructions(*http.Request) (interface{}, error)
}
