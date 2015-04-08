// Copyright 2015 Yahoo
// Author:  David Leon Gil (dgil@yahoo-inc.com)
// License: Apache 2
package ks

import (
	"net/http"

	"github.com/golang/glog"
)

// TODO: split up middleware as appropriate

func log(handle http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		glog.Infof("%s %s", r.Method, r.URL)
		if r.ContentLength <= 0 || r.ContentLength > maxKeyLen {
			// Bail; we don't want to ReadAll...
			glog.Warningf("request content length invalid: %d", r.ContentLength)
			return
		}
		handle(w, r)
	}
}

func requireAuth(f http.HandlerFunc, forwrite bool) http.HandlerFunc {
	if Config.SkipAuth {
		glog.Errorf("requireAuth: skipping auth due to configuration")
		return func(w http.ResponseWriter, r *http.Request) {
			glog.Infof("NOAUTH: request %+v", r)
			f(w, r)
		}
	}

	return func(w http.ResponseWriter, r *http.Request) {
		// When authentication is required, minimize what's logged to prevent
		// logging usable authentication information.

		// This is where you'd implement some sort of authentication scheme.
		// Sorry, no implementation for Yahoo-external users provided just
		// yet.
		f(w, r)
		return
	}
}
