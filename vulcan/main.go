package main

import (
	"fmt"
	"github.com/golang/glog"
	"net/http"
	"time"
)

func main() {
	options, err := parseOptions()
	if err != nil {
		glog.Fatal("Wrong arguments: ", err)
		return
	}

	glog.Infof("Vulcan is starting with arguments: %#v", options)
	proxy, err := initProxy(options)
	if err != nil {
		glog.Fatal("Failed to init proxy: ", err)
		return
	}
	addr := fmt.Sprintf("%s:%d", options.host, options.httpPort)

	s := &http.Server{
		Addr:           addr,
		Handler:        proxy,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}
	if err != nil {
		glog.Fatalf("Failed to init proxy, error:", err)
	}
	glog.Fatal(s.ListenAndServe())
}
