package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	jsh "github.com/derekdowling/go-json-spec-handler"
	"github.com/didip/tollbooth"
)

// SET the GLOBAL rate limiter
const REQUESTSPERSECOND = 10

var rateLimiter = tollbooth.NewLimiter(REQUESTSPERSECOND, nil).SetIPLookups([]string{"RemoteAddr", "X-Forwarded-For", "X-Real-IP"})

func TestAPIHealthCheck(t *testing.T) {
	testServer := httptest.NewServer(tollbooth.LimitFuncHandler(rateLimiter, healthCheck))
	defer testServer.Close()

	client := testServer.Client()
	req, err := http.NewRequest("GET", testServer.URL, nil)
	if err != nil {
		t.Fatal(err)
	}

	var resp *http.Response
	resp, _ = client.Do(req)

	if status := resp.StatusCode; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	// Check the response body is what we expect.
	expectedBody := `ok`
	output := new(bytes.Buffer)
	output.ReadFrom(resp.Body)
	if output.String() != expectedBody {
		t.Errorf("handler returned unexpected body: got %v want %v",
			output.String(), expectedBody)
	}
}
func TestRateLimiterLessThan10(t *testing.T) {
	testServer := httptest.NewServer(tollbooth.LimitFuncHandler(rateLimiter, standardGETRequest))
	defer testServer.Close()

	client := testServer.Client()
	req, err := http.NewRequest("GET", testServer.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 9; i++ {
		wg.Add(1)
		go func(wg *sync.WaitGroup) {
			defer wg.Done()
			var resp *http.Response
			resp, _ = client.Do(req)
			if resp.StatusCode != 200 {
				t.Errorf("Triggered the Rate Limit: got %v want %v", resp.StatusCode, http.StatusOK)
			}
		}(&wg)
	}
	wg.Wait()
}
func TestRateLimiterExactly10(t *testing.T) {
	testServer := httptest.NewServer(tollbooth.LimitFuncHandler(rateLimiter, standardGETRequest))
	defer testServer.Close()

	client := testServer.Client()
	req, _ := http.NewRequest("GET", testServer.URL, nil)

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(wg *sync.WaitGroup) {
			defer wg.Done()
			var resp *http.Response
			resp, _ = client.Do(req)
			if resp.StatusCode != 200 {
				t.Errorf("Triggered the Rate Limit: got %v want %v", resp.StatusCode, http.StatusOK)
			}
		}(&wg)
	}
	wg.Wait()
}
func TestRateLimiterOver10(t *testing.T) {
	testServer := httptest.NewServer(tollbooth.LimitFuncHandler(rateLimiter, standardGETRequest))
	defer testServer.Close()

	client := testServer.Client()
	req, _ := http.NewRequest("GET", testServer.URL, nil)

	statusCodes := []int{}
	var mutex = &sync.Mutex{}

	var wg sync.WaitGroup
	for i := 0; i < 11; i++ {
		wg.Add(1)
		go func(wg *sync.WaitGroup) {
			defer wg.Done()
			var resp *http.Response
			resp, _ = client.Do(req)
			if resp.StatusCode == 429 {
				mutex.Lock()
				statusCodes = append(statusCodes, resp.StatusCode)
				mutex.Unlock()
			}
		}(&wg)
	}
	wg.Wait()

	if len(statusCodes) == 0 {
		t.Errorf("The Rate Limit was not triggered")
	}
}
func TestValidJSONRequest(t *testing.T) {
	testServer := httptest.NewServer(tollbooth.LimitFuncHandler(rateLimiter, randomJSONPOSTRequest))
	defer testServer.Close()

	payload := map[string]string{
		"some": "data",
	}
	jsonReq, _ := json.Marshal(payload)
	client := testServer.Client()
	req, _ := http.NewRequest("POST", testServer.URL, bytes.NewBuffer(jsonReq))

	var resp *http.Response
	var err error
	resp, err = client.Do(req)

	if status := resp.StatusCode; status != http.StatusCreated {
		t.Error(err)
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusCreated)
	}

	// Check the response body is what we expect.
	bodyJSON, _ := json.MarshalIndent(payload, "", "\t")
	expectedBody := string(bodyJSON)
	output := new(bytes.Buffer)
	output.ReadFrom(resp.Body)
	// Trim endline that server places automatically
	outputString := strings.TrimSuffix(output.String(), "\n")
	if outputString != expectedBody {
		t.Errorf("handler returned unexpected body: got\n %v want\n %v",
			outputString, expectedBody)
	}
}
func TestInvalidJSONRequest(t *testing.T) {
	testServer := httptest.NewServer(tollbooth.LimitFuncHandler(rateLimiter, randomJSONPOSTRequest))
	defer testServer.Close()

	client := testServer.Client()
	// create some improper JSON request
	req, _ := http.NewRequest("POST", testServer.URL, bytes.NewBuffer([]byte(`{"some":"msg}`)))

	var resp *http.Response
	var err error
	resp, err = client.Do(req)

	// Check that we get the right code
	if status := resp.StatusCode; status != http.StatusBadRequest {
		t.Error(err)
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusBadRequest)
	}

	// Check the response body is what we expect. (Trim newline from server)
	expectedBody := `{"err":"Sorry, there seems to be an error with your JSON formatting."}`
	output := new(bytes.Buffer)
	output.ReadFrom(resp.Body)
	outputStr := strings.TrimSuffix(output.String(), "\n")
	if outputStr != expectedBody {
		t.Errorf("handler returned unexpected body: got %v want %v",
			outputStr, expectedBody)
	}
}
func TestValidJSONAPIRequest(t *testing.T) {
	testServer := httptest.NewServer(tollbooth.LimitFuncHandler(rateLimiter, JSONAPIPOSTRequest))
	defer testServer.Close()

	payload := `{"data": {"type": "articles","id": "1"}}`
	client := testServer.Client()
	req, _ := http.NewRequest("POST", testServer.URL, bytes.NewBuffer([]byte(payload)))
	req.Header.Set("Content-Type", jsh.ContentType)

	var resp *http.Response
	var err error
	resp, err = client.Do(req)

	if status := resp.StatusCode; status != http.StatusCreated {
		t.Error(err)
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusCreated)
	}

	// Check the response body is what we expect.
	// req2, _ := http.NewRequest("POST", testServer.URL, bytes.NewBuffer([]byte(payload)))
	// req2.Header.Set("Content-Type", jsh.ContentType)
	// rr := httptest.NewRecorder()
	// object, _ := jsh.ParseObject(req2)
	// jsh.Send(rr, req2, object)
	// expected := new(bytes.Buffer)
	// expected.ReadFrom(rr.Body)

	output := new(bytes.Buffer)
	output.ReadFrom(resp.Body)
	// Trim endline that server places automatically
	expectedString := strings.TrimSuffix(string([]byte(
		`{
 "jsonapi": {
  "version": "1.1"
 },
 "data": {
  "type": "articles",
  "id": "1"
 }
}`)), "\n")
	// expectedString := strings.TrimSuffix(expected.String(), "\n")
	outputString := strings.TrimSuffix(output.String(), "\n")
	if outputString != expectedString {
		t.Errorf("handler returned unexpected body: got\n %v want\n %v",
			outputString, expectedString)
	}
}
func TestInvalidJSONAPIRequest(t *testing.T) {
	testServer := httptest.NewServer(tollbooth.LimitFuncHandler(rateLimiter, JSONAPIPOSTRequest))
	client := testServer.Client()
	defer testServer.Close()

	// 1. test for valid payload, invalid header
	validPayload := `{"data": {"type": "articles","id": "1"}}`
	req1, _ := http.NewRequest("POST", testServer.URL, bytes.NewBuffer([]byte(validPayload)))
	var resp1 *http.Response
	resp1, _ = client.Do(req1)

	if status := resp1.StatusCode; status != http.StatusNotAcceptable {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusNotAcceptable)
	}

	// 2. test for invalid payload, with valid header
	invalidPayload := `{"data": { }}`
	req2, _ := http.NewRequest("POST", testServer.URL, bytes.NewBuffer([]byte(invalidPayload)))
	req2.Header.Set("Content-Type", jsh.ContentType)

	var resp2 *http.Response
	var err2 error
	resp2, err2 = client.Do(req2)

	if status := resp2.StatusCode; status != http.StatusNotAcceptable {
		t.Error(err2)
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusNotAcceptable)
	}

	// Check the response body is what we expect.
	req2copy, _ := http.NewRequest("POST", testServer.URL, bytes.NewBuffer([]byte(invalidPayload)))
	req2copy.Header.Set("Content-Type", jsh.ContentType)
	rr := httptest.NewRecorder()
	object, _ := jsh.ParseObject(req2copy)
	jsh.Send(rr, req2copy, object)
	expected := new(bytes.Buffer)
	expected.ReadFrom(rr.Body)

	output := new(bytes.Buffer)
	output.ReadFrom(resp2.Body)
	// Trim endline that server places automatically
	expectedString := strings.TrimSuffix(expected.String(), "\n")
	outputString := strings.TrimSuffix(output.String(), "\n")
	if outputString != expectedString {
		t.Errorf("handler returned unexpected body: got\n %v want\n %v",
			outputString, expectedString)
	}

	// 3. test for invalid JSON, valid header
	// Check the response body is what we expect.
	invalidPayload2 := `{"data": {"type": "articles","id": "1"}`
	req3, _ := http.NewRequest("POST", testServer.URL, bytes.NewBuffer([]byte(invalidPayload2)))
	req3.Header.Set("Content-Type", jsh.ContentType)

	var resp3 *http.Response
	var err3 error
	resp3, err3 = client.Do(req3)

	if status := resp3.StatusCode; status != http.StatusInternalServerError {
		t.Error(err3)
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusInternalServerError)
	}

	// rr := httptest.NewRecorder()
	// object, _ := jsh.ParseObject(req3)
	// jsh.Send(rr, req3, object)
	// expected := new(bytes.Buffer)
	// expected.ReadFrom(rr.Body)

	// output := new(bytes.Buffer)
	// output.ReadFrom(resp3.Body)
	// // Trim endline that server places automatically
	// expectedString := strings.TrimSuffix(expected.String(), "\n")
	// outputString := strings.TrimSuffix(output.String(), "\n")
	// if outputString != expectedString {
	// 	t.Errorf("handler returned unexpected body: got\n %v want\n %v",
	// 		outputString, expectedString)
	// }
}

// TESTS
// Cover all 3 APIs with tests, what needs to be tested:
// [x] that API works in general (TestAPIHealthCheck)
// [x] that valid JSON is being parsed (TestValidJSONRequest)
// [x] invalid JSON returns error (TestInvalidJSONRequest)
// [x] valid JSONAPI being parsed (TestValidJSONAPIRequest)
// [x] invalid JSONAPI returns error (TestInvalidJSONAPIRequest)
// [x] rate limit works:
// -- less than 10 hits per second (TestRateLimiterLessThan10)
// -- exact 10 hits per second (TestRateLimiterExactly10)
// -- more than 10 hits per second - should fail with "Too many requests" http code (TestRateLimiterOver10)
