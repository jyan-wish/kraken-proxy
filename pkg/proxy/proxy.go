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

func TransformRequest(r *http.Request, newAddr string) *http.Request {
	if r.Method == "GET" {
		imgManifest := regexp.MustCompile("^(/v2/)(?P<Name>.*)(/manifests/)(?P<Reference>.*)")
		imgBlob := regexp.MustCompile("^(/v2/)(?P<Name>.*)(/blobs/)(?P<Digest>.*)")
		newUri := r.URL.Path
		if imgManifest.MatchString(r.URL.Path) {
			match := imgManifest.FindStringSubmatch(r.URL.Path)
			newUri = fmt.Sprintf("%s%s/%s%s%s", match[1], r.Host, match[2], match[3], match[4])
		}
		if imgBlob.MatchString(r.URL.Path) {
			match := imgBlob.FindStringSubmatch(r.URL.Path)
			newUri = fmt.Sprintf("%s%s/%s%s%s", match[1], r.Host, match[2], match[3], match[4])
		}
		if newUri != r.URL.Path {
			newUrl := fmt.Sprintf("http://%s%s", newAddr, newUri)
			newReq, err := http.NewRequest(r.Method, newUrl, r.Body)
			if err != nil {
				fmt.Println("Error Tranforming Request: ", err)
				return nil
			}
			return newReq
		} else {
			return nil
		}
	}
	return nil
}

func GenerateProxy(conf *config.Config) *mitm.Proxy {
	cert, err := genCA()
	if err != nil {
		panic(err)
	}
	tlsconf := &tls.Config{
		InsecureSkipVerify: true,
	}
	tp := httpcache.NewMemoryCacheTransport()
	tp.MarkCachedResponses = true
	p := &mitm.Proxy{
		CA:              cert,
		TLSClientConfig: tlsconf,
		TLSServerConfig: tlsconf,
		Wrap: func(upstream http.Handler) http.Handler {
			rp := upstream.(*httputil.ReverseProxy)
			rp.Transport = tp
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				cr := &codeRecorder{ResponseWriter: w}

				newReq := TransformRequest(r, fmt.Sprintf("%s:%s", conf.DesinationHost, conf.DestinationPort))
				if newReq != nil {
					res, err := tp.RoundTrip(newReq)
					if err == nil && res.StatusCode == 200 {
						log.Println("Successfully rerouted to alternative registry")
						for key := range res.Header {
							cr.Header().Add(key, res.Header.Get(key))
						}
						cr.WriteHeader(res.StatusCode)
						io.Copy(cr, res.Body)
						if cr.code != 200 {
							rp.ServeHTTP(cr, r)
						}
					} else {
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
	return p
}
