package collector

import (
	"github.com/jcelliott/lumber"
	"github.com/mu-box/microbox-logtap"
	"io"
	"io/ioutil"
	"net"
	"net/http"
)

// create and return a http handler that can be dropped into an api.
func GenerateHttpCollector(kind string, l *logtap.Logtap) http.HandlerFunc {
	headerName := "X-" + kind + "-Id"
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			return
		}
		logLevel := lumber.LvlInt(r.Header.Get("X-Log-Level"))
		header := r.Header.Get(headerName)
		if header == "" {
			header = kind
		}
		l.Publish(header, logLevel, string(body))
	}
}

func StartHttpCollector(kind, address string, l *logtap.Logtap) (io.Closer, error) {
	httpListener, err := net.Listen("tcp", address)
	if err != nil {
		return nil, err
	}

	go http.Serve(httpListener, GenerateHttpCollector(kind, l))
	return httpListener, nil
}
