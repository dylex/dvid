/*
	This file contains functions useful for testing DVID in other packages.
	Unfortunately, due to the way Go handles compilation of *_test.go files,
	these functions cannot be in server_test.go since they will be unavailable
	to test files in external packages.  So these functions are exported and
	contain the "Test" keyword.
*/

package server

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/janelia-flyem/dvid/dvid"
)

// TestHTTPResponse returns a response from a test run of the DVID server.
// Use TestHTTP if you just want the response body bytes.
func TestHTTPResponse(t *testing.T, method, urlStr string, payload io.Reader) *httptest.ResponseRecorder {
	req, err := http.NewRequest(method, urlStr, payload)
	if err != nil {
		t.Fatalf("Unsuccessful %s on %q: %v\n", method, urlStr, err)
	}
	resp := httptest.NewRecorder()
	ServeSingleHTTP(resp, req)
	return resp
}

// TestHTTP returns the response body bytes for a test request, making sure any response has
// status OK.
func TestHTTP(t *testing.T, method, urlStr string, payload io.Reader) []byte {
	resp := TestHTTPResponse(t, method, urlStr, payload)
	if resp.Code != http.StatusOK {
		var retstr string
		if resp.Body != nil {
			retbytes, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				t.Errorf("Error trying to read response body from request %q: %v\n", urlStr, err)
			} else {
				retstr = string(retbytes)
			}
		}
		t.Fatalf("Bad server response (%d) to %s %q: %s\n", resp.Code, method, urlStr, retstr)
	}
	return resp.Body.Bytes()
}

// TestBadHTTP expects a HTTP response with an error status code.
func TestBadHTTP(t *testing.T, method, urlStr string, payload io.Reader) {
	req, err := http.NewRequest(method, urlStr, payload)
	if err != nil {
		t.Fatalf("Unsuccessful %s on %q: %v\n", method, urlStr, err)
	}
	w := httptest.NewRecorder()
	ServeSingleHTTP(w, req)
	if w.Code == http.StatusOK {
		t.Fatalf("Expected bad server response to %s on %q, got %d instead.\n", method, urlStr, w.Code)
	}
}

func CreateTestInstance(t *testing.T, uuid dvid.UUID, typename, name string, config dvid.Config) {
	config.Set("typename", typename)
	config.Set("dataname", name)
	jsonData, err := config.MarshalJSON()
	if err != nil {
		t.Fatalf("Unable to make JSON for instance creation: %v\n", config)
	}
	apiStr := fmt.Sprintf("%srepo/%s/instance", WebAPIPath, uuid)
	TestHTTP(t, "POST", apiStr, bytes.NewBuffer(jsonData))
}

func CreateTestSync(t *testing.T, uuid dvid.UUID, name string, syncs ...string) {
	url := fmt.Sprintf("%snode/%s/%s/sync", WebAPIPath, uuid, name)
	msg := fmt.Sprintf(`{"sync": "%s"}`, strings.Join(syncs, ","))
	TestHTTP(t, "POST", url, strings.NewReader(msg))
}

func CreateTestReplaceSync(t *testing.T, uuid dvid.UUID, name string, syncs ...string) {
	url := fmt.Sprintf("%snode/%s/%s/sync?replace=true", WebAPIPath, uuid, name)
	msg := fmt.Sprintf(`{"sync": "%s"}`, strings.Join(syncs, ","))
	TestHTTP(t, "POST", url, strings.NewReader(msg))
}
