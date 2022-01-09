package wsp

import (
	"fmt"
	"log"
	"net/http"
)

type HTTPResponse struct {
	StatusCode    int
	Header        http.Header
	ContentLength int64
}

func SerializeHTTPResponse() (r *HTTPResponse) {
	r = new(HTTPResponse)
	r.Header = make(http.Header)
	return
}

func NewHTTPResponse() (r *HTTPResponse) {
	r = new(HTTPResponse)
	r.Header = make(http.Header)
	return
}

func ProxyError(w http.ResponseWriter, err error) {
	log.Println(err)
	http.Error(w, err.Error(), 526)
}

func ProxyErrorf(w http.ResponseWriter, format string, args ...interface{}) {
	ProxyError(w, fmt.Errorf(format, args...))
}
