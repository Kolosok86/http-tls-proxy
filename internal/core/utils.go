package core

import (
	"bufio"
	"crypto/sha256"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/Kolosok86/http"
	utls "github.com/refraction-networking/utls"
)

var blacklist = []string{
	"proxy-protocol",
	"proxy-node-escape",
	"proxy-downgrade",
	"proxy-tls-setup",
	"proxy-tls",
}

func itsChrome(userAgent string) bool {
	return strings.Contains(strings.ToLower(userAgent), "chrome")
}

func Hijack(hijackable interface{}) (net.Conn, *bufio.ReadWriter, error) {
	hj, ok := hijackable.(http.Hijacker)
	if !ok {
		return nil, nil, errors.New("connection doesn't support hijacking")
	}
	conn, rw, err := hj.Hijack()
	if err != nil {
		return nil, nil, err
	}
	var emptytime time.Time
	err = conn.SetDeadline(emptytime)
	if err != nil {
		conn.Close()
		return nil, nil, err
	}
	return conn, rw, nil
}

func ReadRequest(reader *bufio.Reader, scheme string) (*http.Request, error) {
	r, err := http.ReadRequest(reader)
	if err != nil {
		return nil, err
	}

	r.RequestURI, r.URL.Scheme, r.URL.Host = "", scheme, r.Host
	return r, nil
}

func StringToSpec(ja3 string, userAgent string, proto []string) (*utls.ClientHelloSpec, error) {
	chrome := itsChrome(userAgent)
	extMap := genMap(proto)
	tokens := strings.Split(ja3, ",")

	// Check if we have enough tokens for a valid JA3 string
	if len(tokens) < 5 {
		return nil, fmt.Errorf("invalid JA3 string: expected 5 components, got %d", len(tokens))
	}

	// Safely access tokens with bounds checking
	var ciphers, extensions, curves []string

	if len(tokens) > 1 {
		ciphers = strings.Split(tokens[1], "-")
	}
	if len(tokens) > 2 {
		extensions = strings.Split(tokens[2], "-")
	}
	if len(tokens) > 3 {
		curves = strings.Split(tokens[3], "-")
	}
	if len(curves) == 1 && curves[0] == "" {
		curves = []string{}
	}

	var pointFormats []string
	if len(tokens) > 4 {
		pointFormats = strings.Split(tokens[4], "-")
		if len(pointFormats) == 1 && pointFormats[0] == "" {
			pointFormats = []string{}
		}
	}

	// Parse Curves
	var targetCurves []utls.CurveID
	targetCurves = append(targetCurves, utls.CurveID(utls.GREASE_PLACEHOLDER))
	for _, c := range curves {
		cid, err := strconv.ParseUint(c, 10, 16)
		if err != nil {
			return nil, err
		}

		targetCurves = append(targetCurves, utls.CurveID(cid))
	}

	extMap["10"] = &utls.SupportedCurvesExtension{Curves: targetCurves}

	// Parse point formats
	var targetPointFormats []byte
	for _, p := range pointFormats {
		pid, err := strconv.ParseUint(p, 10, 8)
		if err != nil {
			return nil, err
		}
		targetPointFormats = append(targetPointFormats, byte(pid))
	}

	extMap["11"] = &utls.SupportedPointsExtension{SupportedPoints: targetPointFormats}

	// Build extensions list
	var exts []utls.TLSExtension
	// Optionally Add Chrome Grease Extension
	if chrome {
		exts = append(exts, &utls.UtlsGREASEExtension{})
	}

	for _, e := range extensions {
		te, ok := extMap[e]
		if !ok {
			return nil, nil
		}

		// Optionally add Chrome Grease Extension
		if e == "21" && chrome {
			exts = append(exts, &utls.UtlsGREASEExtension{})
		}

		exts = append(exts, te)
	}

	// Build CipherSuites
	var suites []uint16
	// Optionally Add Chrome Grease Extension
	if chrome {
		suites = append(suites, utls.GREASE_PLACEHOLDER)
	}

	for _, c := range ciphers {
		cid, err := strconv.ParseUint(c, 10, 16)
		if err != nil {
			return nil, err
		}

		suites = append(suites, uint16(cid))
	}

	return &utls.ClientHelloSpec{
		CipherSuites:       suites,
		CompressionMethods: []byte{0x00},
		Extensions:         exts,
		GetSessionID:       sha256.Sum256,
	}, nil
}

func genMap(proto []string) (extMap map[string]utls.TLSExtension) {
	extMap = map[string]utls.TLSExtension{
		"0": &utls.SNIExtension{},
		"5": &utls.StatusRequestExtension{},
		"13": &utls.SignatureAlgorithmsExtension{
			SupportedSignatureAlgorithms: []utls.SignatureScheme{
				utls.ECDSAWithP256AndSHA256,
				utls.PSSWithSHA256,
				utls.PKCS1WithSHA256,
				utls.ECDSAWithP384AndSHA384,
				utls.PSSWithSHA384,
				utls.PKCS1WithSHA384,
				utls.PSSWithSHA512,
				utls.PKCS1WithSHA512,
			},
		},
		"16": &utls.ALPNExtension{
			AlpnProtocols: proto,
		},
		"17": &utls.GenericExtension{Id: 17},
		"18": &utls.SCTExtension{},
		"21": &utls.UtlsPaddingExtension{GetPaddingLen: utls.BoringPaddingStyle},
		"22": &utls.GenericExtension{Id: 22},
		"23": &utls.UtlsExtendedMasterSecretExtension{},
		"27": &utls.UtlsCompressCertExtension{
			Algorithms: []utls.CertCompressionAlgo{utls.CertCompressionBrotli},
		},
		"28": &utls.FakeRecordSizeLimitExtension{}, //Limit: 0x4001
		"34": &utls.GenericExtension{Id: 34},
		"35": &utls.SessionTicketExtension{},
		"41": &utls.GenericExtension{Id: 41},
		"43": &utls.SupportedVersionsExtension{Versions: []uint16{
			utls.GREASE_PLACEHOLDER,
			utls.VersionTLS13,
			utls.VersionTLS12,
		}},
		"44": &utls.CookieExtension{},
		"45": &utls.PSKKeyExchangeModesExtension{Modes: []uint8{
			utls.PskModeDHE,
		}},
		"49": &utls.GenericExtension{Id: 49}, // post_handshake_auth
		"50": &utls.GenericExtension{Id: 50}, // signature_algorithms_cert
		"51": &utls.KeyShareExtension{KeyShares: []utls.KeyShare{
			{Group: utls.CurveID(utls.GREASE_PLACEHOLDER), Data: []byte{0}},
			{Group: utls.X25519},
		}},
		"13172": &utls.NPNExtension{},
		"17513": &utls.ApplicationSettingsExtension{
			SupportedProtocols: []string{
				"h2",
			},
		},
		"65281": &utls.RenegotiationInfoExtension{
			Renegotiation: utls.RenegotiateOnceAsClient,
		},
	}

	return
}

func RemoveServiceHeaders(req *http.Request, opts []string) {
	list := append(blacklist, opts...)

	for _, key := range list {
		if ok := req.Header.Get(key); ok == "" {
			continue
		}

		req.HeaderOrder.Del(key)
		req.Header.Del(key)
	}
}
