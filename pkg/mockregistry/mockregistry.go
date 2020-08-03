package mockregistry

import (
	"fmt"
	"net/http"
)

type MockServer struct {
	Port      int
	Responses map[string]string
}

func CreateTestServer(port int) *MockServer {
	responses := make(map[string]string)
	return &MockServer{port, responses}
}

func (ms *MockServer) AddResponse(uri string, resp string) {
	ms.Responses[uri] = resp
}

func (ms *MockServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if resp, ok := ms.Responses[r.RequestURI]; ok {
		fmt.Fprintf(w, resp)
		return
	}
	fmt.Fprint(w, "")
	//Throw some error here?
}
