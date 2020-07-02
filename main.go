package main

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"flag"
	"fmt"
	"io/ioutil"
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
	krakenRegistryHost = flag.String("kraken-registry-host", "localhost", "host of kraken registry")
	krakenRegistryPort = flag.Int("kraken-registry-port", 8081, "port of kraken registry")
	tlsCert            = flag.String("tls-cert", "", "file container the tls certificate")
	tlsKey             = flag.String("tls-key", "", "file container the private key")
)

type codeRecorder struct {
	http.ResponseWriter
	code int
}

func (w *codeRecorder) WriteHeader(code int) {
	w.ResponseWriter.WriteHeader(code)
	w.code = code
}

func getCA() (*tls.Certificate, error) {
	if *tlsCert == "" {
		log.Fatalf("Missing required flag tls-cert")
	}
	if *tlsKey == "" {
		log.Fatalf("Missing required flag tls-key")
	}
	cert, err := ioutil.ReadFile(*tlsCert)
	if err != nil {
		return nil, err
	}
	key, err := ioutil.ReadFile(*tlsKey)
	if err != nil {
		return nil, err
	}
	certBlock, _ := pem.Decode([]byte(cert))
	keyBlock, _ := pem.Decode([]byte(key))
	certPem := pem.EncodeToMemory(&pem.Block{
		Type:  certBlock.Type,
		Bytes: certBlock.Bytes,
	})
	keyPem := pem.EncodeToMemory(&pem.Block{
		Type:  keyBlock.Type,
		Bytes: keyBlock.Bytes,
	})
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
			newUrl := fmt.Sprintf("https://%s:%d%s", *krakenRegistryHost, *krakenRegistryPort, r.RequestURI)
			r.URL, _ = url.Parse(newUrl)
			r.Host = r.URL.Host
		}
	}
}

func main() {
	flag.Parse()
	cert, err := getCA()
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
