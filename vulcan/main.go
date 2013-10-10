package main

import (
	"fmt"
	"github.com/golang/glog"
	"github.com/mailgun/vulcan"
	"io/ioutil"
	"net/http"
	"os"
	"time"
)

func main() {
	options, err := parseOptions()
	if err != nil {
		glog.Fatal("Wrong arguments: ", err)
		return
	}

	// Write process id to a file, if asked. This is extremely useful
	// for various monitoring tools
	if options.pidPath != "" {
		pidBytes := []byte(fmt.Sprintf("%d", os.Getpid()))
		err = ioutil.WriteFile(options.pidPath, pidBytes, 0644)
		if err != nil {
			panic(err)
		}
	}

	if vulcan.LogDir() != "" {
		glog.Infof("Starting log cleanup go routine with period: %s", options.cleanupPeriod)
		go func() {
			t := time.Tick(options.cleanupPeriod)
			for {
				select {
				case <-t:
					glog.Infof("Start cleaning up the logs")
					err := vulcan.CleanupLogs()
					if err != nil {
						glog.Errorf("Failed to clean up the logs: %s, shutting down goroutine", err)
						return
					}
				}
			}
		}()
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
