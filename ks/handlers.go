// Copyright 2015 Yahoo
// Author:  David Leon Gil (dgil@yahoo-inc.com)
// License: Apache 2
package ks

import (
	"io/ioutil"
	"net/http"
	"time"

	"encoding/json"
	"github.com/golang/glog"
	"github.com/gorilla/mux"
	"github.com/yahoo/keyshop/yenc"
)

const (
	// FIXME(OSS): Maybe. This is more than reasonable for
	// EC keys. If you want to support fancy PQCrypto, perhaps
	// not enough?
	maxKeyLen = 4096
)

var (
	bucket = []byte("keys")
)

var (
	// Post handles requests to /v1/k/{userid}/{deviceid}
	// The body of the request is the key to associate with
	// this user's device {deviceid}
	// It requires that
	//    {userid}
	//    body.userid
	// are identical.
	Post      = log(requireAuth(post, true))
	Get       = log(requireAuth(get, false))
	DeleteAll = log(requireAuth(deleteAll, true))
)

func deleteAll(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	userid := vars["userid"]
	w.WriteHeader(ks.DeleteAll(userid))
	return
}

func post(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	userid, deviceid := vars["userid"], vars["deviceid"]

	// Read the key
	enc, err := ioutil.ReadAll(r.Body)
	if err != nil {
		glog.Warningf("couldn't read the full request: %s", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	var key []byte
	encKey := string(enc)

	// FIXME(OSS): This does not necessarily guarantee that the output
	// is terminal-safe.
	glog.V(4).Infof("got body of %s", encKey)
	key, err = yenc.RawURL64.DecodeString(encKey)
	if err != nil {
		glog.Warningf("invalid base64: %s", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	// Check that the key's userid and userid are the same,
	//   FIXME(OSS): This is a stub for some other authentication
	//   mechanism.
	// also validating that the key is valid.
	if !validKeyForUser(userid, userid, key) {
		glog.Warningf("was not a valid key for userid %s", userid)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	// Prepare the DKey for signing.
	prekey := &DKey{
		UserID:    userid,
		DeviceID:  deviceid,
		Key:       encKey,
		Timestamp: time.Now().UTC().Unix(),
	}
	data, err := json.Marshal(prekey)
	if err != nil {
		glog.Errorf("error marshalling prekey: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	glog.V(4).Infof("marshalled prekey: %s", data)
	dkey, err := ka.Sign(data)
	if err != nil {
		glog.Errorf("error getting signature from kauth: %s", err)
	}

	status := ks.NewOrUpdate([]byte(userid), []byte(deviceid), dkey)
	if status != http.StatusOK {
		glog.Infof("post: status %s", status)
	}
	h := w.Header()
	h.Set("Content-Type", "application/jws")
	w.WriteHeader(status)
	w.Write(dkey)
	return
}

// GET /<userid>
// Returns (checks are sequential):
//   401 StatusUnauthorized: If the Bouncer auth is invalid or not present
//   404 StatusNotFound    : If no public keys are registered for the userid
//   5xx                   : Random server issues that should never occur
func get(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	userid, ok := vars["userid"]
	if !ok {
		glog.Errorf("hunh? no userid passed to Get; this shouldn't be possible")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// FIXME(OSS): Check that you're willing to process keys for this email address.
	if !verifyAuth(userid, r) {
		// Note that in this case, no signed statement is made.
		w.WriteHeader(http.StatusNotFound)
		return
	}

	keys, status := ks.Get(userid)
	switch status {
	case http.StatusOK:
		break
	case http.StatusNotFound:
		// We sign a statement that there are no registered keys.
		// TODO: This should be cached up to some max-freshness period.
		keys = make(map[string]string)
		w.WriteHeader(http.StatusNotFound)
	default:
		w.WriteHeader(status)
		return
	}

	ukeys := &UKeys{
		Timestamp: time.Now().UTC().Unix(),
		UserID:    userid,
		Keys:      keys,
	}
	data, err := json.Marshal(ukeys)
	glog.Infof("data: %s", data)
	if err != nil {
		glog.Errorf("error marshalling keys: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	signed, err := ka.Sign(data)
	if err != nil {
		glog.Errorf("error marshaling signed keybundle: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	h := w.Header()
	h.Set("Content-Type", "application/jws")
	glog.Infof("signed: %s", signed)
	w.Write(signed)
}
