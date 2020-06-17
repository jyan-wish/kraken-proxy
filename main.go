package main

import (
	"crypto/tls"
	"crypto/x509"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"

	"github.com/gregjones/httpcache"
	"github.com/kr/mitm"
)

var (
	listenPort         = flag.Int("listen-port", 6000, "port to listen on")
	redirectHost       = flag.String("redirect-host", "127.0.0.1", "host to redirect calls")
	redirectPort       = flag.Int("redirect-port", 1234, "port to redirect calls")
	krakenRegistryHost = flag.String("kraken-registry-host", "localhost", "host of kraken registry")
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

func genCA() (cert tls.Certificate, err error) {
	certPEM, keyPEM, err := mitm.GenCA("test")
	if err != nil {
		return tls.Certificate{}, err
	}
	cert, err = tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return tls.Certificate{}, err
	}
	cert.Leaf, err = x509.ParseCertificate(cert.Certificate[0])
	return cert, err
}

func transformRequest(r *http.Request) (*http.Request, error) {
	reqUrl := r.URL
	reqUrl.Host = fmt.Sprintf("%s:%d", *redirectHost, *redirectPort)
	newReq, err := http.NewRequest("POST", reqUrl.String(), nil)
	if err != nil {
		return nil, err
	}
	q := r.URL.Query()
	image := q.Get("fromImage")
	if image != "" {
		newImage := fmt.Sprintf("%s:%d/%s", *krakenRegistryHost, *krakenRegistryPort, image)
		q.Set("fromImage", newImage)
	}
	newReq.URL.RawQuery = q.Encode()
	return newReq, nil
}

func main() {
	cert, err := genCA()
	if err != nil {
		panic(err)
	}
	tp := httpcache.NewMemoryCacheTransport()
	tp.MarkCachedResponses = true
	p := &mitm.Proxy{
		CA: &cert,
		Wrap: func(upstream http.Handler) http.Handler {
			// Hack in the caching transport for this RP
			rp := upstream.(*httputil.ReverseProxy)
			rp.Transport = tp
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				cr := &codeRecorder{ResponseWriter: w}
				newReq, err := transformRequest(r)
				if err != nil {
					log.Printf("Error: %v\n", err)
				}
				rp.ServeHTTP(cr, newReq)
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
