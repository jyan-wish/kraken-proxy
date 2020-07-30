package mockregistry

import (
	"fmt"
	"log"
	"net/http"
)

type MockServer struct {
	responses map[string]string
}

func CreateTestServer() *MockServer {
	responses := make(map[string]string)
	return &MockServer{responses}
}

func (ms *MockServer) AddResponse(uri string, resp string) {
	ms.responses[uri] = resp
}

func (ms *MockServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if resp, ok := ms.responses[r.RequestURI]; ok {
		fmt.Fprintf(w, resp)
		return
	}
	fmt.Fprint(w, "")
}

func (ms *MockServer) StartServer() {
	log.Println("Starting Test Server")
	s := &http.Server{
		Addr:    ":5454",
		Handler: ms,
	}
	s.ListenAndServeTLS("./certs/cert.pem", "./certs/key.pem")
}
