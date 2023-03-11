package core

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"

	utls "github.com/refraction-networking/utls"
	"golang.org/x/net/proxy"

	"github.com/Kolosok86/http"
	"github.com/Kolosok86/http/http2"
)

var errProtocolNegotiated = errors.New("protocol negotiated")

type roundTripper struct {
	sync.Mutex

	JA3       string
	UserAgent string
	Downgrade bool

	connections map[string]net.Conn
	transports  map[string]http.RoundTripper

	dialer proxy.ContextDialer
}

func (rt *roundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	addr := rt.getDialTLSAddr(req)

	if _, ok := rt.transports[addr]; !ok {
		if err := rt.getTransport(req, addr); err != nil {
			return nil, err
		}
	}

	return rt.transports[addr].RoundTrip(req)
}

func (rt *roundTripper) getTransport(req *http.Request, addr string) error {
	switch strings.ToLower(req.URL.Scheme) {
	case "http":
		rt.transports[addr] = &http.Transport{DialContext: rt.dialer.DialContext, DisableKeepAlives: true}
		return nil
	case "https":
	default:
		return fmt.Errorf("invalid URL scheme: [%v]", req.URL.Scheme)
	}

	_, err := rt.dialTLS(context.Background(), "tcp", addr)
	switch err {
	case errProtocolNegotiated:
	case nil:
		// Should never happen.
		panic("dialTLS returned no error when determining cached transports")
	default:
		return err
	}

	return nil
}

func (rt *roundTripper) dialTLS(ctx context.Context, network, addr string) (net.Conn, error) {
	rt.Lock()
	defer rt.Unlock()

	// If we have the connection from when we determined the HTTPS
	// cached transports to use, return that.
	if conn := rt.connections[addr]; conn != nil {
		delete(rt.connections, addr)
		return conn, nil
	}

	rawConn, err := rt.dialer.DialContext(ctx, network, addr)
	if err != nil {
		return nil, err
	}

	var host string
	if host, _, err = net.SplitHostPort(addr); err != nil {
		host = addr
	}

	helloAgent := utls.HelloChrome_106_Shuffle
	if rt.JA3 != "" {
		helloAgent = utls.HelloCustom
	}

	conn := utls.UClient(rawConn, &utls.Config{
		InsecureSkipVerify: true,
		ServerName:         host,
	},
		helloAgent,
	)

	if err = rt.setSpec(conn); err != nil {
		return nil, err
	}

	if err = conn.Handshake(); err != nil {
		_ = conn.Close()

		if err.Error() == "tls: curve preferences includes unsupported curve" {
			return nil, fmt.Errorf("conn.Handshake() error for tls 1.3 (please retry request): %+v", err)
		}

		return nil, fmt.Errorf("uTlsConn.Handshake() error: %+v", err)
	}

	if rt.transports[addr] != nil {
		return conn, nil
	}

	// No http.Transport constructed yet, create one based on the results of ALPN.
	if conn.ConnectionState().NegotiatedProtocol == http2.NextProtoTLS {
		rt.transports[addr] = &http2.Transport{
			DialTLS: rt.dialTLSHTTP2,

			// set chrome initial params
			HeaderTableSize:      65536,
			InitialWindowSize:    6291456,
			InitMaxReadFrameSize: 262144,
		}
	} else {
		// Assume the remote peer is speaking HTTP 1.x + TLS.
		rt.transports[addr] = &http.Transport{DialTLSContext: rt.dialTLS}
	}

	// Stash the connection just established for use servicing the
	// actual request (should be near-immediate).
	rt.connections[addr] = conn

	return nil, errProtocolNegotiated
}

func (rt *roundTripper) setSpec(conn *utls.UConn) error {
	if rt.JA3 == "" {
		return nil
	}

	proto := []string{"h2", "http/1.1"}
	if rt.Downgrade {
		proto = proto[1:]
	}

	spec, err := StringToSpec(rt.JA3, rt.UserAgent, proto)
	if err != nil {
		return err
	}

	if err = conn.ApplyPreset(spec); err != nil {
		return err
	}

	return nil
}

func (rt *roundTripper) dialTLSHTTP2(network, addr string, _ *utls.Config) (net.Conn, error) {
	return rt.dialTLS(context.Background(), network, addr)
}

func (rt *roundTripper) getDialTLSAddr(req *http.Request) string {
	host, port, err := net.SplitHostPort(req.URL.Host)
	if err == nil {
		return net.JoinHostPort(host, port)
	}

	return net.JoinHostPort(req.URL.Host, "443")
}

func NewRoundTripper(JA3, UserAgent string, downgrade bool) http.RoundTripper {
	return &roundTripper{
		dialer: proxy.Direct,

		JA3:       JA3,
		UserAgent: UserAgent,
		Downgrade: downgrade,

		transports:  make(map[string]http.RoundTripper),
		connections: make(map[string]net.Conn),
	}
}
