package js

import (
	"fmt"
	"github.com/golang/glog"
	. "github.com/mailgun/vulcan/command"
	"github.com/mailgun/vulcan/discovery"
	"github.com/robertkrimen/otto"
	"net/http"
)

type JsController struct {
	DiscoveryService discovery.Service
	CodeGetter       CodeGetter
}

func (ctrl *JsController) GetInstructions(req *http.Request) (interface{}, error) {
	var instr interface{}
	err := fmt.Errorf("Not implemented")
	defer func() {
		if r := recover(); r != nil {
			glog.Errorf("Recovered:", r)
			err = fmt.Errorf("Internal js error")
			instr = nil
		}
	}()
	code, err := ctrl.CodeGetter.GetCode()
	if err != nil {
		return nil, err
	}
	Otto := otto.New()
	ctrl.registerBuiltins(Otto)

	_, err = Otto.Run(code)
	if err != nil {
		return nil, err
	}
	value, err := Otto.Get("handle")
	if err != nil {
		return nil, err
	}
	return ctrl.resultToInstructions(value)
}

func (ctrl *JsController) resultToInstructions(value otto.Value) (interface{}, error) {
	if !value.IsFunction() {
		return nil, fmt.Errorf("Result should be a function, got %v", value)
	}
	out, err := value.Call(value)
	if err != nil {
		glog.Infof("Call resulted in failure %#v", err)
		return nil, err
	}
	obj, err := out.Export()
	if err != nil {
		glog.Infof("Failed to extract response %#v", err)
		return nil, err
	}
	return NewCommandFromObj(obj)
}

func (ctrl *JsController) registerBuiltins(o *otto.Otto) {
	ctrl.addDiscoveryService(o)
}

func (ctrl *JsController) addDiscoveryService(o *otto.Otto) {
	return
	o.Set("discover", func(call otto.FunctionCall) otto.Value {
		right, _ := call.Argument(0).ToString()
		value, err := ctrl.DiscoveryService.Get(right)
		glog.Infof("Got %v, %s", value, err)
		result, _ := o.ToValue(value)
		return result
	})
}
