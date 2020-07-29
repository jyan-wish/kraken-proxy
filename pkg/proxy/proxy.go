package proxy

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"regexp"

	"github.com/gregjones/httpcache"
	"github.com/jyan-wish/kraken-proxy/pkg/config"
	"github.com/kr/mitm"
)

type codeRecorder struct {
	http.ResponseWriter
	code int
	req  *http.Request
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

func transformRequest(r *http.Request, config *config.Config) (*http.Request, error) {
	if r.Method == "GET" {
		imgManifest := regexp.MustCompile("(/v2/)(?P<Name>.*)(/manifests/)(?P<Reference>.*)")
		imgBlob := regexp.MustCompile("(/v2/)(?P<Name>.*)(/blobs/)(?P<Digest>.*)")
		newUri := r.RequestURI
		if imgManifest.MatchString(r.RequestURI) {
			match := imgManifest.FindStringSubmatch(r.RequestURI)
			newUri = fmt.Sprintf("%s%s/%s%s%s", match[1], r.Host, match[2], match[3], match[4])
		}
		if imgBlob.MatchString(r.RequestURI) {
			match := imgBlob.FindStringSubmatch(r.RequestURI)
			newUri = fmt.Sprintf("%s%s/%s%s%s", match[1], r.Host, match[2], match[3], match[4])
		}
		if newUri != r.RequestURI {
			newUrl := fmt.Sprintf("https://%s:%d%s", config.DesinationHost, config.DestinationPort, newUri)
			newReq, err := http.NewRequest(r.Method, newUrl, r.Body)
			if err != nil {
				return nil, err
			}
			return newReq, nil
		} else {
			return nil, nil
		}
	}
	return nil, nil
}

func StartProxy(conf *config.Config) {
	cert, err := genCA()
	if err != nil {
		panic(err)
	}
	confi := &tls.Config{
		InsecureSkipVerify: true,
	}
	tp := httpcache.NewMemoryCacheTransport()
	tp.MarkCachedResponses = true
	p := &mitm.Proxy{
		CA:              cert,
		TLSClientConfig: confi,
		TLSServerConfig: confi,
		Wrap: func(upstream http.Handler) http.Handler {
			rp := upstream.(*httputil.ReverseProxy)
			rp.Transport = tp
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				cr := &codeRecorder{ResponseWriter: w}
				newReq, err := transformRequest(r, conf)
				if err != nil {
					fmt.Printf("Error when transforming request: %+v\n", err)
				}
				if newReq != nil {
					res, err := tp.RoundTrip(newReq)
					if err == nil && res.StatusCode == 200 {
						log.Println("Successfully rerouted to alternative registry")
						for key, _ := range res.Header {
							cr.Header().Add(key, res.Header.Get(key))
						}
						io.Copy(cr, res.Body)
					} else {
						fmt.Println(res, err)
						log.Println("Unsuccessful reroute, falling back to upstream")
						rp.ServeHTTP(cr, r)
					}
				} else {
					rp.ServeHTTP(cr, r)
				}
				log.Println("Got Status:", cr.code)
			})
		},
	}
	s := &http.Server{
		Addr:    fmt.Sprintf(":%d", conf.ListenPort),
		Handler: p,
	}
	log.Printf("Serving on port %d\n", conf.ListenPort)
	log.Fatal(s.ListenAndServe())
}
