package main

import (
	"crypto/tls"
	"crypto/x509"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"regexp"

	"github.com/gregjones/httpcache"
	"github.com/kr/mitm"
)

var (
	listenPort         = flag.Int("listen-port", 6000, "port to listen on")
	krakenRegistryPort = flag.Int("kraken-registry-port", 8081, "port of kraken registry")
)

type codeRecorder struct {
	http.ResponseWriter
	code int
}

func (w *codeRecorder) WriteHeader(code int) {
	w.ResponseWriter.WriteHeader(code)
	w.code = code
}

func genCA() (*tls.Certificate, error) {
	certPem, keyPem, err := mitm.GenCA("proxy")
	if err != nil {
		return nil, err
	}
	finalCert, err := tls.X509KeyPair(certPem, keyPem)
	if err != nil {
		return nil, err
	}
	finalCert.Leaf, err = x509.ParseCertificate(finalCert.Certificate[0])
	return &finalCert, err
}

func transformRequest(r *http.Request) {
	if r.Method == "GET" {
		imgManifest := regexp.MustCompile("/v2/(?P<Name>.*)/manifests/(?P<Reference>.*)")
		imgBlob := regexp.MustCompile("/v2/(?P<Name>.*)/blobs/(?P<Digest>.*)")
		if imgManifest.MatchString(r.RequestURI) || imgBlob.MatchString(r.RequestURI) {
			newUrl := fmt.Sprintf("https://localhost:%d%s", *krakenRegistryPort, r.RequestURI)
			r.URL, _ = url.Parse(newUrl)
			r.Host = r.URL.Host
		}
	}
}

func main() {
	flag.Parse()
	cert, err := genCA()
	if err != nil {
		panic(err)
	}
	tp := httpcache.NewMemoryCacheTransport()
	tp.MarkCachedResponses = true
	p := &mitm.Proxy{
		CA: cert,
		Wrap: func(upstream http.Handler) http.Handler {
			// Hack in the caching transport for this RP
			rp := upstream.(*httputil.ReverseProxy)
			rp.Transport = tp
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				cr := &codeRecorder{ResponseWriter: w}
				transformRequest(r)
				rp.ServeHTTP(cr, r)
				log.Println("Got Status:", cr.code)
			})
		},
	}
	s := &http.Server{
		Addr:    fmt.Sprintf(":%d", *listenPort),
		Handler: p,
	}
	log.Printf("Serving on port %d\n", *listenPort)
	log.Fatal(s.ListenAndServe())
}
