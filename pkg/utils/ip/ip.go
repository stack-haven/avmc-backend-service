package ip

import (
	"context"
	"errors"
	"net"
	"net/http"
	"strings"

	"github.com/go-kratos/kratos/v2/transport"
)

const (
	XRealIP     = "X-Real-IP"
	XForwardFor = "X-Forward-For"
)

// GetIP returns request real ip.
func GetIP(r *http.Request) (string, error) {
	ip := r.Header.Get(XRealIP)
	if net.ParseIP(ip) != nil {
		return ip, nil
	}

	ip = r.Header.Get(XForwardFor)
	for _, i := range strings.Split(ip, ",") {
		if net.ParseIP(i) != nil {
			return i, nil
		}
	}

	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return "", err
	}

	if net.ParseIP(ip) != nil {
		return ip, nil
	}

	return "", errors.New("no valid ip found")
}

func FormContext(ctx context.Context) string {
	if header, ok := transport.FromServerContext(ctx); ok {
		ip := header.RequestHeader().Get(XRealIP)
		if net.ParseIP(ip) != nil {
			return ip
		}

		forwardFor := strings.Split(header.RequestHeader().Get(XForwardFor), ",")
		num := len(forwardFor)
		if num > 0 {
			ip = forwardFor[num-1]
		}

		if net.ParseIP(ip) != nil {
			return ip
		}
	}
	return "0.0.0.0"
}
