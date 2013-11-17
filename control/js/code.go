package js

import (
	"io/ioutil"
)

type CodeGetter interface {
	GetCode() (string, error)
}

type StringGetter struct {
	Code string
}

func (sg *StringGetter) GetCode() (string, error) {
	return sg.Code, nil
}

type FileGetter struct {
	Path string
}

func (fg *FileGetter) GetCode() (string, error) {
	code, err := ioutil.ReadFile(fg.Path)
	if err != nil {
		return "", err
	} else {
		return string(code), err
	}
}
