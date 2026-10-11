package main

/*
#include <stdlib.h>
*/
import "C"
import (
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"

	"github.com/tylergannon/skgo/nativehost"
)

// This host owns presentation transport only. It never opens a Gimbal project
// or durable store and never starts the independent workspace server.
var host = nativehost.New(func(origin string) (http.Handler, error) {
	target := os.Getenv("GIMBAL_SERVER_URL")
	if target == "" {
		target = "http://127.0.0.1:8080"
	}
	server, err := url.Parse(target)
	if err != nil || server.Host == "" {
		return nil, fmt.Errorf("invalid GIMBAL_SERVER_URL")
	}
	proxy := &httputil.ReverseProxy{Rewrite: func(r *httputil.ProxyRequest) {
		r.SetURL(server)
		r.Out.Header.Set("X-Gimbal-Desktop", "macos")
		if r.In.Header.Get("Origin") != "" {
			r.Out.Header.Set("Origin", server.String())
		}
		if ref, err := url.Parse(r.In.Header.Get("Referer")); err == nil && ref.Host != "" {
			ref.Scheme = server.Scheme
			ref.Host = server.Host
			r.Out.Header.Set("Referer", ref.String())
		}
	}, ErrorHandler: func(w http.ResponseWriter, r *http.Request, e error) {
		http.Error(w, "Gimbal server unavailable. Start the independent server and reconnect; pending recording text is not saved.", http.StatusServiceUnavailable)
	}}
	return proxy, nil
})

//export SKGoHostStart
func SKGoHostStart() *C.char {
	origin, err := host.Start()
	if err != nil {
		log.Print(err)
		return nil
	}
	return C.CString(origin)
}

//export SKGoHostRecover
func SKGoHostRecover() C.int {
	if host.Recover() != nil {
		return 0
	}
	return 1
}

//export SKGoHostStop
func SKGoHostStop() { _ = host.Stop() }
func main()         {}
