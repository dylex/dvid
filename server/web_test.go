package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/janelia-flyem/dvid/dvid"
	"github.com/janelia-flyem/dvid/tests"
)

func createRepo(t *testing.T) dvid.UUID {
	apiStr := fmt.Sprintf("%srepos", WebAPIPath)
	r := TestHTTP(t, "POST", apiStr, nil)
	var jsonResp map[string]interface{}

	if err := json.Unmarshal(r, &jsonResp); err != nil {
		t.Fatalf("Unable to unmarshal repo creation response: %s\n", string(r))
	}
	v, ok := jsonResp["root"]
	if !ok {
		t.Fatalf("No 'root' metadata returned: %s\n", string(r))
	}
	uuid, ok := v.(string)
	if !ok {
		t.Fatalf("Couldn't cast returned 'root' data (%v) into string.\n", v)
	}
	return dvid.UUID(uuid)
}

func TestLog(t *testing.T) {
	tests.UseStore()
	defer tests.CloseStore()

	uuid := createRepo(t)

	// Post a log
	payload := bytes.NewBufferString(`{"log": ["line1", "line2", "some more stuff in a line"]}`)
	apiStr := fmt.Sprintf("%snode/%s/log", WebAPIPath, uuid)
	TestHTTP(t, "POST", apiStr, payload)

	// Verify it was saved.
	r := TestHTTP(t, "GET", apiStr, nil)
	jsonResp := make(map[string]interface{})
	if err := json.Unmarshal(r, &jsonResp); err != nil {
		t.Fatalf("Unable to unmarshal log response: %s\n", string(r))
	}
	if len(jsonResp) != 1 {
		t.Errorf("Bad log return: %s\n", string(r))
	}
	v, ok := jsonResp["log"]
	if !ok {
		t.Fatalf("No 'log' data returned: %s\n", string(r))
	}
	data, ok := v.([]string)
	if !ok {
		t.Errorf("Expected []string in logs, got instead: %v\n", v)
	}
	if len(data) != 3 {
		t.Errorf("Got wrong # of lines in log: %v\n", data)
	}
	if data[0] != "line1" {
		t.Errorf("Got bad log line: %q\n", data[0])
	}
	if data[1] != "line2" {
		t.Errorf("Got bad log line: %q\n", data[0])
	}
	if data[2] != "some more stuff in a line" {
		t.Errorf("Got bad log line: %q\n", data[0])
	}

	// Add some more to log
	payload = bytes.NewBufferString(`{"log": ["line4", "line5"]}`)
	apiStr = fmt.Sprintf("%snode/%s/log", WebAPIPath, uuid)
	TestHTTP(t, "POST", apiStr, payload)

	// Verify it was appended.
	r = TestHTTP(t, "GET", apiStr, nil)
	jsonResp = make(map[string]interface{})
	if err := json.Unmarshal(r, &jsonResp); err != nil {
		t.Fatalf("Unable to unmarshal log response: %s\n", string(r))
	}
	if len(jsonResp) != 1 {
		t.Errorf("Bad log return: %s\n", string(r))
	}
	v, ok = jsonResp["log"]
	if !ok {
		t.Fatalf("No 'log' data returned: %s\n", string(r))
	}
	data, ok = v.([]string)
	if !ok {
		t.Errorf("Expected []string in logs, got instead: %v\n", v)
	}
	if len(data) != 5 {
		t.Errorf("Got wrong # of lines in log: %v\n", data)
	}
	if data[3] != "line4" {
		t.Errorf("Got bad log line: %q\n", data[0])
	}
	if data[4] != "line5" {
		t.Errorf("Got bad log line: %q\n", data[0])
	}
}

func TestLock(t *testing.T) {

}

func TestBranch(t *testing.T) {

}
