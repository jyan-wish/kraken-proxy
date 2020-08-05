package proxy

import (
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/jyan-wish/kraken-proxy/pkg/config"
	"github.com/jyan-wish/kraken-proxy/pkg/mockregistry"
)

func randString(n int) string {
	const options = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	txt := make([]byte, n)
	for i := range txt {
		txt[i] = byte(options[rand.Intn(len(options))])
	}
	return string(txt)
}

func imgManifestUri() string {
	imgName := randString(8)
	reference := randString(8)
	return fmt.Sprintf("/v2/%s/manifests/%s", imgName, reference)
}

func imgLayerUri() string {
	imgName := randString(8)
	reference := randString(8)
	return fmt.Sprintf("/v2/%s/blobs/%s", imgName, reference)
}

func proxyClient(u string) *http.Client {
	proxyUrl, err := url.Parse(u)
	if err != nil {
		return nil
	}
	return &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyURL(proxyUrl),
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
		Timeout: time.Second * 5,
	}
}

func genProxy(destination *httptest.Server, upstream *httptest.Server) *httptest.Server {
	destUrl, _ := url.Parse(destination.URL)
	upUrl, _ := url.Parse(upstream.URL)
	conf := &config.Config{
		ListenPort:      upUrl.Port(),
		DestinationPort: destUrl.Port(),
		DesinationHost:  destUrl.Hostname(),
	}
	return httptest.NewServer(GenerateProxy(conf))
}

func transformedUri(oldReq *http.Request, serverUrl string) (string, error) {
	krakenUrl, err := url.Parse(serverUrl)
	if err != nil {
		return "", err
	}
	newReq := TransformRequest(oldReq, krakenUrl.Host)
	if newReq == nil {
		return "", fmt.Errorf("Expected to transform request: %s", oldReq.URL)
	}
	return newReq.URL.Path, nil
}

func handleError(e error, t *testing.T) {
	if e == nil {
		return
	}
	t.Fatalf("Got unexpected error: %+v\n", e)
}

func TestImageLayerTransformRequest(t *testing.T) {
	uri := imgLayerUri()
	originalReq, err := http.NewRequest("GET", "https://google.com:1234"+uri, nil)
	handleError(err, t)
	newHost := "host2.com:1234"
	newReq := TransformRequest(originalReq, newHost)
	if newReq == nil {
		t.Fatalf("Expected to transform request: %s\n", originalReq.URL)
	}
	if newReq.URL.Host != newHost {
		t.Fatalf("Expected transformed request host %s into %s but got %s\n", originalReq.URL.Host, newHost, newReq.URL.Host)
	}
}

func TestImageManifestTransformRequest(t *testing.T) {
	uri := imgManifestUri()
	originalReq, err := http.NewRequest("GET", "https://google.com:1234"+uri, nil)
	handleError(err, t)
	newHost := "host2.com:1234"
	newReq := TransformRequest(originalReq, newHost)
	if newReq == nil {
		t.Fatalf("Expected to transform request: %s\n", originalReq.URL)
	}
	if newReq.URL.Host != newHost {
		t.Fatalf("Expected transformed request host %s into %s but got %s\n", originalReq.URL.Host, newHost, newReq.URL.Host)
	}
}

func TestNoTransformRequest(t *testing.T) {
	originalReq, err := http.NewRequest("GET", "https://host1.com:2341/path", nil)
	handleError(err, t)
	newReq := TransformRequest(originalReq, "host2.com:9452")
	if newReq != nil {
		t.Fatalf("Expected non-transformed request, transformed request %s to %s", originalReq.URL, newReq.URL)
	}
}

func TestImageLayerReroute(t *testing.T) {
	kraken := mockregistry.CreateTestServer()
	krakenServer := httptest.NewServer(kraken)
	defer krakenServer.Close()

	backup := mockregistry.CreateTestServer()
	backupServer := httptest.NewServer(backup)
	defer backupServer.Close()

	prox := genProxy(krakenServer, backupServer)
	defer prox.Close()

	c := proxyClient(prox.URL)

	uri := imgLayerUri()
	resString := randString(8)

	req, err := http.NewRequest("GET", backupServer.URL+uri, nil)
	handleError(err, t)
	transformedUri, err := transformedUri(req, krakenServer.URL)
	handleError(err, t)
	kraken.AddResponse(transformedUri, resString)

	resp, err := c.Do(req)
	handleError(err, t)
	defer resp.Body.Close()
	b, err := ioutil.ReadAll(resp.Body)
	if string(b) != resString {
		t.Fatalf("Expected response %s got %s", resString, string(b))
	}
}

func TestImageManifestReroute(t *testing.T) {
	kraken := mockregistry.CreateTestServer()
	krakenServer := httptest.NewServer(kraken)
	defer krakenServer.Close()

	backup := mockregistry.CreateTestServer()
	backupServer := httptest.NewServer(backup)
	defer backupServer.Close()

	prox := genProxy(krakenServer, backupServer)
	defer prox.Close()

	c := proxyClient(prox.URL)

	uri := imgManifestUri()
	resString := randString(8)

	req, err := http.NewRequest("GET", backupServer.URL+uri, nil)
	transformedUri, err := transformedUri(req, krakenServer.URL)
	handleError(err, t)
	kraken.AddResponse(transformedUri, resString)

	resp, err := c.Do(req)
	handleError(err, t)
	defer resp.Body.Close()
	b, err := ioutil.ReadAll(resp.Body)
	if string(b) != resString {
		t.Fatalf("Expected response %s got %s", resString, string(b))
	}
}

func TestImageLayerBackup(t *testing.T) {
	kraken := mockregistry.CreateTestServer()
	krakenServer := httptest.NewServer(kraken)
	defer krakenServer.Close()

	backup := mockregistry.CreateTestServer()
	backupServer := httptest.NewServer(backup)
	defer backupServer.Close()

	prox := genProxy(krakenServer, backupServer)
	defer prox.Close()

	c := proxyClient(prox.URL)

	uri := imgLayerUri()
	resString := randString(8)
	backup.AddResponse(uri, resString)

	req, err := http.NewRequest("GET", backupServer.URL+uri, nil)
	handleError(err, t)

	resp, err := c.Do(req)
	handleError(err, t)
	defer resp.Body.Close()
	b, err := ioutil.ReadAll(resp.Body)
	if string(b) != resString {
		t.Fatalf("Expected response %s got %s", resString, string(b))
	}
}

func TestImageManifestBackup(t *testing.T) {
	kraken := mockregistry.CreateTestServer()
	krakenServer := httptest.NewServer(kraken)
	defer krakenServer.Close()

	backup := mockregistry.CreateTestServer()
	backupServer := httptest.NewServer(backup)
	defer backupServer.Close()

	prox := genProxy(krakenServer, backupServer)
	defer prox.Close()

	c := proxyClient(prox.URL)

	uri := imgManifestUri()
	resString := randString(8)
	backup.AddResponse(uri, resString)

	req, err := http.NewRequest("GET", backupServer.URL+uri, nil)
	handleError(err, t)

	resp, err := c.Do(req)
	handleError(err, t)
	defer resp.Body.Close()
	b, err := ioutil.ReadAll(resp.Body)
	if string(b) != resString {
		t.Fatalf("Expected response %s got %s", resString, string(b))
	}
}
