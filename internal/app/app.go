package app

import (
	"fmt"
	"strings"
	"time"

	"github.com/Kolosok86/http"
	"github.com/kolosok86/proxy/internal/core"
)

const BAD_REQ_MSG = "Bad Request\n"

type ProxyHandler struct {
	timeout   time.Duration
	logger    *core.Logger
	transport http.RoundTripper
}

func NewProxyHandler(timeout time.Duration, logger *core.Logger) *ProxyHandler {
	return &ProxyHandler{
		transport: &http.Transport{},
		timeout:   timeout,
		logger:    logger,
	}
}

func (s *ProxyHandler) ServeHTTP(wr http.ResponseWriter, req *http.Request) {
	isConnect := strings.ToUpper(req.Method) == "CONNECT"
	if (req.URL.Host == "" || req.URL.Scheme == "" && !isConnect) && req.ProtoMajor < 2 ||
		req.Host == "" && req.ProtoMajor == 2 {
		http.Error(wr, BAD_REQ_MSG, http.StatusBadRequest)
		return
	}

	s.logger.Info("Request: %v %v %v %v", req.RemoteAddr, req.Proto, req.Method, req.URL)

	if !isConnect {
		http.Error(wr, BAD_REQ_MSG, http.StatusBadRequest)
	} else {
		s.HandleTunnel(wr, req)
	}
}

func (s *ProxyHandler) HandleTunnel(wr http.ResponseWriter, req *http.Request) {
	if req.ProtoMajor == 2 {
		s.logger.Error("Unsupported protocol version: %s", req.Proto)
		http.Error(wr, "Unsupported protocol version.", http.StatusBadRequest)
		return
	}

	// Upgrade client connection
	local, reader, err := core.Hijack(wr)
	if err != nil {
		s.logger.Error("Can't hijack client connection: %v", err)
		http.Error(wr, "Can't hijack client connection", http.StatusInternalServerError)
		return
	}

	defer local.Close()

	// Inform client connection is built
	fmt.Fprintf(local, "HTTP/%d.%d 200 OK\r\n\r\n", req.ProtoMajor, req.ProtoMinor)

	request, err := core.ReadRequest(reader.Reader, "http")
	if err != nil {
		s.logger.Error("HTTP read error: %v", err)
		http.Error(wr, "Server Read Error", http.StatusInternalServerError)
		return
	}

	// get all header - settings
	scheme := request.Header.Get("proxy-protocol")
	downgrade := request.Header.Get("proxy-downgrade") != ""
	node := request.Header.Get("proxy-node-escape")
	setup := request.Header.Get("proxy-tls-setup")
	hash := request.Header.Get("proxy-tls")

	if scheme != "http" && scheme != "https" {
		scheme = "https"
	}

	agent := request.UserAgent()
	request.URL.Scheme = scheme

	client := &http.Client{
		Transport: core.NewRoundTripper(hash, setup, agent, downgrade),
		Timeout:   10 * time.Second,
	}

	var additional []string
	if node != "" {
		additional = append(additional, "Connection")
	}

	core.RemoveServiceHeaders(request, additional)

	resp, err := client.Do(request)
	if err != nil {
		s.logger.Error("HTTP fetch error: %v", err)
		http.Error(wr, "Server Request Error", http.StatusInternalServerError)
		return
	}

	defer resp.Body.Close()

	s.logger.Info("%v %v %v %v", req.RemoteAddr, req.Method, req.URL, resp.Status)

	err = resp.Write(local)
	if err != nil {
		s.logger.Error("HTTP dump error: %v", err)
		http.Error(wr, "Received Bad Response", http.StatusInternalServerError)
	}
}
