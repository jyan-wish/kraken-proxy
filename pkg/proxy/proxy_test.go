package proxy

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

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
		},
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

func TestImage(t *testing.T) {
	mr := mockregistry.CreateTestServer(5454)
	mrs := httptest.NewServer(mr)
	defer mrs.Close()

	mr1 := mockregistry.CreateTestServer(2000)
	mrs1 := httptest.NewServer(mr1)
	defer mrs1.Close()

	prox := genProxy(mrs, mrs1)
	defer prox.Close()

	c := proxyClient(prox.URL)

	uri := imgManifestUri()
	r := randString(8)
	mr1.AddResponse(uri, r)

	resp, err := c.Get(mrs1.URL + uri)

	if err != nil {
		t.Fatal(err)
	}
	b, err := ioutil.ReadAll(resp.Body)
	if string(b) != r {
		t.Fatalf("Expected response %s got %s", r, string(b))
	}
}
