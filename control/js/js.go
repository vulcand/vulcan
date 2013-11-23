package js

import (
	"fmt"
	"github.com/golang/glog"
	. "github.com/mailgun/vulcan/command"
	"github.com/mailgun/vulcan/discovery"
	"github.com/mailgun/vulcan/netutils"
	"github.com/robertkrimen/otto"
	"net/http"
)

type JsController struct {
	DiscoveryService discovery.Service
	CodeGetter       CodeGetter
}

func (ctrl *JsController) ConvertError(req *http.Request, inError error) (response *netutils.HttpError) {
	response = netutils.NewHttpError(http.StatusInternalServerError)
	defer func() {
		if r := recover(); r != nil {
			glog.Errorf("Recovered:", r)
		}
	}()
	code, err := ctrl.CodeGetter.GetCode()
	if err != nil {
		glog.Errorf("Error getting code: %s", err)
		return response
	}
	Otto := otto.New()
	ctrl.registerBuiltins(Otto)

	_, err = Otto.Run(code)
	if err != nil {
		glog.Errorf("Error running code: %s", err)
		return response
	}
	handler, err := Otto.Get("handleError")
	if err != nil {
		glog.Infof("Missing error handler: %s", err)
		converted, err := errorFromJs(errorToJs(err))
		if err != nil {
			return response
		}
		return converted
	}
	obj := errorToJs(inError)
	jsObj, err := Otto.ToValue(obj)
	if err != nil {
		glog.Errorf("Error: %s", err)
		return response
	}
	jsRequest, err := requestToJs(req)
	if err != nil {
		return response
	}
	jsRequestValue, err := Otto.ToValue(jsRequest)
	if err != nil {
		return response
	}
	out, err := ctrl.callHandler(handler, jsRequestValue, jsObj)
	if err != nil {
		glog.Errorf("Error: %s", err)
		return response
	}
	converted, err := errorFromJs(out)
	if err != nil {
		glog.Errorf("Failed to convert error: %s", err)
		return response
	}
	return converted
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
	handler, err := Otto.Get("handle")
	if err != nil {
		return nil, err
	}
	jsRequest, err := requestToJs(req)
	if err != nil {
		return nil, err
	}
	jsObj, err := Otto.ToValue(jsRequest)
	if err != nil {
		return nil, err
	}
	instr, err = ctrl.callHandler(handler, jsObj)
	if err != nil {
		return nil, err
	}
	return NewCommandFromObj(instr)
}

func (ctrl *JsController) callHandler(handler otto.Value, params ...interface{}) (interface{}, error) {
	if !handler.IsFunction() {
		return nil, fmt.Errorf("Result should be a function, got %v", handler)
	}
	out, err := handler.Call(handler, params...)
	if err != nil {
		glog.Infof("Call resulted in failure %#v", err)
		return nil, err
	}
	obj, err := out.Export()
	if err != nil {
		glog.Infof("Failed to extract response %#v", err)
		return nil, err
	}
	return obj, nil
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
