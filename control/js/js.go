package js

import (
	"fmt"
	"github.com/golang/glog"
	"github.com/mailgun/vulcan/client"
	. "github.com/mailgun/vulcan/command"
	"github.com/mailgun/vulcan/discovery"
	"github.com/mailgun/vulcan/netutils"
	"github.com/robertkrimen/otto"
	"net/http"
	"runtime/debug"
)

type JsController struct {
	DiscoveryService discovery.Service
	CodeGetter       CodeGetter
	Client           client.Client
}

func (ctrl *JsController) ConvertError(req *http.Request, inError error) (response *netutils.HttpError, err error) {
	response = netutils.NewHttpError(http.StatusInternalServerError)
	err = fmt.Errorf("Internal error")
	defer func() {
		if r := recover(); r != nil {
			glog.Errorf("Recovered: %v %s", r, debug.Stack())
		}
	}()
	code, err := ctrl.CodeGetter.GetCode()
	if err != nil {
		glog.Errorf("Error getting code: %s", err)
		return response, err
	}
	Otto := otto.New()
	ctrl.registerBuiltins(Otto)

	_, err = Otto.Run(code)
	if err != nil {
		glog.Errorf("Error running code: %s", err)
		return response, err
	}
	handler, err := Otto.Get("handleError")
	if err != nil {
		return nil, err
	}
	if handler.IsUndefined() {
		glog.Infof("Missing error handler: %s", err)
		converted, err := errorFromJs(errorToJs(inError))
		if err != nil {
			glog.Errorf("Failed to convert error: %s", err)
			return nil, err
		}
		return converted, nil
	}
	obj := errorToJs(inError)
	jsObj, err := Otto.ToValue(obj)
	if err != nil {
		glog.Errorf("Error: %s", err)
		return nil, err
	}
	jsRequest, err := requestToJs(req)
	if err != nil {
		return nil, err
	}
	jsRequestValue, err := Otto.ToValue(jsRequest)
	if err != nil {
		return nil, err
	}
	out, err := ctrl.callHandler(handler, jsRequestValue, jsObj)
	if err != nil {
		glog.Errorf("Error: %s", err)
		return nil, err
	}
	converted, err := errorFromJs(out)
	if err != nil {
		glog.Errorf("Failed to convert error: %s", err)
		return nil, err
	}
	return converted, nil
}

func (ctrl *JsController) GetInstructions(req *http.Request) (interface{}, error) {
	var instr interface{}
	err := fmt.Errorf("Not implemented")
	defer func() {
		if r := recover(); r != nil {
			glog.Errorf("Recovered: %v %s", r, debug.Stack())
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
		glog.Errorf("Call resulted in failure %#v", err)
		return nil, err
	}
	obj, err := out.Export()
	if err != nil {
		glog.Errorf("Failed to extract response %#v", err)
		return nil, err
	}
	return obj, nil
}

func (ctrl *JsController) registerBuiltins(o *otto.Otto) {
	ctrl.addDiscoveryService(o)
	ctrl.addGetter(o)
	ctrl.addInfo(o)
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

func (ctrl *JsController) addInfo(o *otto.Otto) {
	o.Set("info", func(call otto.FunctionCall) otto.Value {
		if len(call.ArgumentList) <= 0 {
			glog.Errorf("GET: Missing arguments")
			return otto.NullValue()
		}
		formatI, err := call.Argument(0).Export()
		if err != nil {
			glog.Errorf("Fail: %s", err)
			return otto.NullValue()
		}
		formatString, err := toString(formatI)
		if err != nil {
			return otto.NullValue()
		}
		if len(call.ArgumentList) <= 1 {
			glog.Infof(formatString)
		} else {
			arguments := make([]interface{}, len(call.ArgumentList)-1)
			for i, val := range call.ArgumentList {
				if i == 0 {
					continue
				}
				obj, err := val.Export()
				if err != nil {
					glog.Errorf("Failed to convert argument: %v", err)
				}
				arguments[i-1] = obj
			}
			glog.Infof(formatString, arguments...)
		}
		return otto.NullValue()
	})
}

func (ctrl *JsController) addGetter(o *otto.Otto) {
	o.Set("get", func(call otto.FunctionCall) otto.Value {
		if len(call.ArgumentList) <= 0 {
			glog.Errorf("GET: Missing arguments")
			return newError(o, fmt.Errorf("GET: missing arguments"))
		}

		// Convert first argument, expect either string with url or list of strings
		upstreamsI, err := call.Argument(0).Export()
		if err != nil {
			glog.Errorf("GET: Failed to export first argument: %s", err)
			return newError(o, err)
		}
		upstreams, err := toStringArray(upstreamsI)
		if err != nil {
			glog.Errorf("GET: Failed to convert upstreams: %s", err)
			return newError(o, err)
		}

		// Second argument may be absent
		var query client.MultiDict
		if len(call.ArgumentList) > 1 {
			queryI, err := call.Argument(1).Export()
			if err != nil {
				glog.Errorf("GET: Failed to export first argument: %s", err)
				return newError(o, err)
			}
			dict, err := toMultiDict(queryI)
			if err != nil {
				glog.Errorf("GET: Failed: %s", err)
				return newError(o, err)
			}
			query = dict
		}

		// Third argument is optional username/password object
		var auth *netutils.BasicAuth
		if len(call.ArgumentList) > 2 {
			queryI, err := call.Argument(2).Export()
			if err != nil {
				glog.Errorf("GET: Failed: %s", err)
				return newError(o, err)
			}
			creds, err := toBasicAuth(queryI)
			if err != nil {
				glog.Errorf("GET: Failed: %s", err)
				return newError(o, err)
			}
			auth = creds
		}
		writer := NewResponseWriter()
		err = ctrl.Client.Get(writer, upstreams, query, auth)
		if err != nil {
			glog.Errorf("GET: Failed: %s", err)
			return newError(o, err)
		}
		reply := writer.ToReply()
		converted, err := o.ToValue(reply)
		if err != nil {
			glog.Errorf("GET: Failed: %s", err)
			return newError(o, err)
		}
		return converted
	})
}
