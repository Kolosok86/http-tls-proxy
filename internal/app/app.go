package app

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"strings"
	"time"

	"github.com/Kolosok86/http"
	"github.com/kolosok86/proxy/internal/core"
)

const (
	BAD_REQ_MSG              = "Bad Request\n"
	UNSUPPORTED_PROTOCOL_MSG = "Unsupported protocol version."
	SERVER_READ_ERROR_MSG    = "Server Read Error"
	SERVER_REQUEST_ERROR_MSG = "Server Request Error"
	HIJACK_ERROR_MSG         = "Can't hijack client connection"

	DEFAULT_SCHEME      = "https"
	HTTP_OK_RESPONSE    = "HTTP/%d.%d 200 OK\r\n\r\n"
	HTTP_ERROR_RESPONSE = "HTTP/1.1 500 Internal Server Error\r\n\r\n%s"
)

// Config contains the proxy configuration
type Config struct {
	Timeout        time.Duration
	MaxConnections int
	AllowedSchemes []string
	LogLevel       int
}

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	return &Config{
		Timeout:        10 * time.Second,
		MaxConnections: 1000,
		AllowedSchemes: []string{"http", "https"},
		LogLevel:       20,
	}
}

// RequestValidator is an interface for request validation
type RequestValidator interface {
	IsValid(req *http.Request, isConnect bool) bool
}

// DefaultValidator is the default validator implementation
type DefaultValidator struct{}

func (v *DefaultValidator) IsValid(req *http.Request, isConnect bool) bool {
	return !((req.URL.Host == "" || req.URL.Scheme == "" && !isConnect) && req.ProtoMajor < 2 ||
		req.Host == "" && req.ProtoMajor == 2)
}

type ProxyHandler struct {
	config    *Config
	logger    *core.Logger
	transport http.RoundTripper
	validator RequestValidator
}

func NewProxyHandler(config *Config, logger *core.Logger) *ProxyHandler {
	if config == nil {
		config = DefaultConfig()
	}

	return &ProxyHandler{
		config:    config,
		transport: &http.Transport{},
		logger:    logger,
		validator: &DefaultValidator{},
	}
}

// NewProxyHandlerWithValidator creates a handler with a custom validator
func NewProxyHandlerWithValidator(config *Config, logger *core.Logger, validator RequestValidator) *ProxyHandler {
	handler := NewProxyHandler(config, logger)
	handler.validator = validator
	return handler
}

func (s *ProxyHandler) ServeHTTP(wr http.ResponseWriter, req *http.Request) {
	isConnect := strings.ToUpper(req.Method) == "CONNECT"

	if !s.validator.IsValid(req, isConnect) {
		s.logger.Error("Invalid request from %v: %v %v", req.RemoteAddr, req.Method, req.URL)
		http.Error(wr, BAD_REQ_MSG, http.StatusBadRequest)
		return
	}

	s.logger.Info("Request: %v %v %v %v", req.RemoteAddr, req.Proto, req.Method, req.URL)

	if !isConnect {
		s.HandleHTTP(wr, req)
	} else {
		s.HandleTunnel(wr, req)
	}
}

func (s *ProxyHandler) HandleHTTP(wr http.ResponseWriter, req *http.Request) {
	// Extract settings from headers
	proxyConfig := s.extractProxyConfig(req)

	// Validate scheme
	if !s.isSchemeAllowed(proxyConfig.scheme) {
		s.logger.Error("Scheme not allowed: %v", proxyConfig.scheme)
		http.Error(wr, "Scheme not allowed", http.StatusBadRequest)
		return
	}

	// Configure the request
	s.setupRequest(req, proxyConfig)
	req.RequestURI = ""

	// Create a client with context
	ctx, cancel := context.WithTimeout(context.Background(), s.config.Timeout)
	defer cancel()

	client := s.createHTTPClient(proxyConfig)

	// Remove service headers
	s.removeServiceHeaders(req, proxyConfig.nodeEscape)

	// Execute the request
	resp, err := client.Do(req.WithContext(ctx))
	if err != nil {
		s.logger.Error("HTTP fetch error: %v", err)
		http.Error(wr, SERVER_REQUEST_ERROR_MSG, http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	s.logger.Info("Response: %v %v %v %v", req.RemoteAddr, req.Method, req.URL, resp.Status)

	// Copy response headers
	for key, values := range resp.Header {
		for _, value := range values {
			wr.Header().Add(key, value)
		}
	}

	// Set status code
	wr.WriteHeader(resp.StatusCode)

	// Copy response body
	if _, err := io.Copy(wr, resp.Body); err != nil {
		s.logger.Error("Error copying response body: %v", err)
		return
	}
}

func (s *ProxyHandler) HandleTunnel(wr http.ResponseWriter, req *http.Request) {
	if req.ProtoMajor == 2 {
		s.logger.Error("Unsupported protocol version: %s", req.Proto)
		http.Error(wr, UNSUPPORTED_PROTOCOL_MSG, http.StatusBadRequest)
		return
	}

	// Upgrade client connection
	local, reader, err := core.Hijack(wr)
	if err != nil {
		s.logger.Error("Can't hijack client connection: %v", err)
		http.Error(wr, HIJACK_ERROR_MSG, http.StatusInternalServerError)
		return
	}

	defer func() {
		if cerr := local.Close(); cerr != nil {
			s.logger.Error("Error closing connection: %v", cerr)
		}
	}()

	// Inform client connection is built
	if _, err := fmt.Fprintf(local, HTTP_OK_RESPONSE, req.ProtoMajor, req.ProtoMinor); err != nil {
		s.logger.Error("Error writing response: %v", err)
		return
	}

	if err := s.processProxyRequest(local, reader, req); err != nil {
		s.logger.Error("Proxy request processing failed: %v", err)
	}
}

func (s *ProxyHandler) processProxyRequest(local net.Conn, reader *bufio.ReadWriter, originalReq *http.Request) error {
	request, err := core.ReadRequest(reader.Reader, "http")
	if err != nil {
		s.logger.Error("HTTP read error: %v", err)
		fmt.Fprintf(local, HTTP_ERROR_RESPONSE, SERVER_READ_ERROR_MSG)
		return err
	}

	// Extract settings from headers
	proxyConfig := s.extractProxyConfig(request)

	// Validate scheme
	if !s.isSchemeAllowed(proxyConfig.scheme) {
		s.logger.Error("Scheme not allowed: %v", proxyConfig.scheme)
		fmt.Fprintf(local, HTTP_ERROR_RESPONSE, "Scheme not allowed")
		return fmt.Errorf("scheme not allowed: %s", proxyConfig.scheme)
	}

	// Configure the request
	s.setupRequest(request, proxyConfig)

	// Create a client with context
	ctx, cancel := context.WithTimeout(context.Background(), s.config.Timeout)
	defer cancel()

	client := s.createHTTPClient(proxyConfig)

	// Remove service headers
	s.removeServiceHeaders(request, proxyConfig.nodeEscape)

	// Execute the request
	resp, err := client.Do(request.WithContext(ctx))
	if err != nil {
		s.logger.Error("HTTP fetch error: %v", err)
		fmt.Fprintf(local, HTTP_ERROR_RESPONSE, SERVER_REQUEST_ERROR_MSG)
		return err
	}

	defer resp.Body.Close()

	s.logger.Info("Response: %v %v %v %v", originalReq.RemoteAddr, originalReq.Method, originalReq.URL, resp.Status)

	// Send response to client
	if err := resp.Write(local); err != nil {
		s.logger.Error("HTTP dump error: %v", err)
		return err
	}

	return nil
}

type proxyConfig struct {
	scheme     string
	downgrade  bool
	nodeEscape string
	tlsSetup   string
	tlsHash    string
	userAgent  string
}

func (s *ProxyHandler) extractProxyConfig(request *http.Request) proxyConfig {
	scheme := request.Header.Get("proxy-protocol")
	if scheme != "http" && scheme != "https" {
		scheme = DEFAULT_SCHEME
	}

	return proxyConfig{
		scheme:     scheme,
		downgrade:  request.Header.Get("proxy-downgrade") != "",
		nodeEscape: request.Header.Get("proxy-node-escape"),
		tlsSetup:   request.Header.Get("proxy-tls-setup"),
		tlsHash:    request.Header.Get("proxy-tls"),
		userAgent:  request.UserAgent(),
	}
}

func (s *ProxyHandler) setupRequest(request *http.Request, config proxyConfig) {
	request.URL.Scheme = config.scheme
}

func (s *ProxyHandler) createHTTPClient(config proxyConfig) *http.Client {
	return &http.Client{
		Transport: core.NewRoundTripper(config.tlsHash, config.tlsSetup, config.userAgent, config.downgrade),
		Timeout:   s.config.Timeout,
	}
}

func (s *ProxyHandler) removeServiceHeaders(request *http.Request, nodeEscape string) {
	var additional []string
	if nodeEscape != "" {
		additional = append(additional, "Connection")
	}
	core.RemoveServiceHeaders(request, additional)
}

func (s *ProxyHandler) isSchemeAllowed(scheme string) bool {
	for _, allowed := range s.config.AllowedSchemes {
		if scheme == allowed {
			return true
		}
	}

	return false
}
