package mockregistry

import (
	"fmt"
	"net/http"
)

type MockServer struct {
	Responses map[string]string
}

func CreateTestServer() *MockServer {
	responses := make(map[string]string)
	return &MockServer{Responses: responses}
}

func (ms *MockServer) AddResponse(uri string, resp string) {
	ms.Responses[uri] = resp
}

func (ms *MockServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if resp, ok := ms.Responses[r.RequestURI]; ok {
		fmt.Fprintf(w, resp)
		return
	}
	http.Error(w, "Not Found", 404)
}
