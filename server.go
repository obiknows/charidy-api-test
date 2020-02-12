package main

import (
	"encoding/json"
	"log"
	"math/rand"
	"net/http"
	"strconv"
	"time"

	jsh "github.com/derekdowling/go-json-spec-handler"
	"github.com/didip/tollbooth"
)

func main() {
	// Create the rate limiter - (10 req/s)
	// SET the GLOBAL rate limiter
	const REQUESTSPERSECOND = 10

	var rateLimiter = tollbooth.NewLimiter(REQUESTSPERSECOND, nil).SetIPLookups([]string{"RemoteAddr", "X-Forwarded-For", "X-Real-IP"})

	http.Handle("/health", tollbooth.LimitFuncHandler(rateLimiter, healthCheck))
	http.Handle("/", tollbooth.LimitFuncHandler(rateLimiter, standardGETRequest))
	http.Handle("/json", tollbooth.LimitFuncHandler(rateLimiter, randomJSONPOSTRequest))
	http.Handle("/jsonapi", tollbooth.LimitFuncHandler(rateLimiter, JSONAPIPOSTRequest))

	log.Println("Listening on localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

// healthCheck is a regular GET Reqeust that returns 200
func healthCheck(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		// give back a default response
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	default:
		http.Error(w, "Sorry, Only GET methods are currently supported", http.StatusMethodNotAllowed)
	}
}

// standardGETRequest is a regular GET Reqeust that does general
func standardGETRequest(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		// seed the random number generator
		s1 := rand.NewSource(time.Now().UnixNano())
		r1 := rand.New(s1)
		randWait := (r1.Intn(1000) + 500)

		// wait 500ms - 1.5s --> simulate some very expensive database call
		time.Sleep(time.Duration(randWait) * time.Millisecond)

		// give back a default response
		w.WriteHeader(http.StatusOK)
		w.Header().Set("X-Compute-Response-Time", strconv.Itoa(randWait)+"ms")
		w.Write([]byte("Here's a Nice Web Page, or some Data"))
	default:
		http.Error(w, "Sorry, Only GET methods are currently supported", http.StatusMethodNotAllowed)
	}
}

// randomJSONPOSTRequest handles regular posted JSON data
func randomJSONPOSTRequest(w http.ResponseWriter, r *http.Request) {
	// set the content-type
	w.Header().Set("Content-Type", "application/json")
	switch r.Method {
	case "POST":

		var data map[string]interface{}
		err := json.NewDecoder(r.Body).Decode(&data)

		if err != nil {
			errMsg := map[string]string{"err": "Sorry, there seems to be an error with your JSON formatting."}
			jsonErrMsg, _ := json.Marshal(errMsg)
			w.Header().Set("Content-Type", "application/json")
			http.Error(w, string(jsonErrMsg), http.StatusBadRequest)
		} else {
			// set the Indent to pretty print raw JSON response
			w.WriteHeader(http.StatusCreated)
			encoder := json.NewEncoder(w)
			encoder.SetIndent("", "\t")
			encoder.Encode(data)
		}

	default:
		http.Error(w, "Sorry, Only POST methods are currently supported", http.StatusMethodNotAllowed)
	}
}

// JSONAPIPOSTRequest follow the JSON API spec
func JSONAPIPOSTRequest(w http.ResponseWriter, r *http.Request) {
	// set the content-type
	w.Header().Set("Content-Type", jsh.ContentType)
	switch r.Method {
	case "POST":
		//
		object, err := jsh.ParseObject(r)
		if err != nil {
			// jsh returns API friendly errors, that are easy to respond with
			jsh.Send(w, r, err)
			return
		}
		jsh.Send(w, r, object)

	default:
		http.Error(w, "Sorry, Only POST methods are currently supported", http.StatusMethodNotAllowed)
	}
}
