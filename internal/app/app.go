package app

import (
	"encoding/json"
	"errors"
	"fmt"
	nhttp "net/http"
	"strings"

	http "github.com/Danny-Dasilva/fhttp"
	"github.com/kolosok86/proxy/internal/core"
	"github.com/quotpw/tlsHttpClient/tlsHttpClient"
)

func HandleReq(w nhttp.ResponseWriter, r *nhttp.Request) {
	defer r.Body.Close()

	if r.Method != nhttp.MethodPost {
		nhttp.Error(w, "Method not allowed", nhttp.StatusMethodNotAllowed)
		return
	}

	request := &core.RequestParams{}
	err := json.NewDecoder(r.Body).Decode(request)
	if err != nil {
		nhttp.Error(w, err.Error(), nhttp.StatusBadRequest)
		return
	}

	if request.URL == "" {
		nhttp.Error(w, "Set url first", nhttp.StatusBadRequest)
		return
	}

	fmt.Printf("Receive request to url %s:\n", request.URL)

	client := tlsHttpClient.New()
	executor := client.R()

	if request.Proxy != "" && setProxy(client, request.Proxy) != nil {
		nhttp.Error(w, err.Error(), nhttp.StatusBadRequest)
		return
	}

	client.SetDisableRedirect(request.DisableRedirect)
	if request.Headers != nil {
		client.SetHeaders(request.Headers)
	}

	if request.UserAgent != "" {
		client.SetHeader("User-Agent", request.UserAgent)
	}

	if request.Params != nil {
		client.SetQueryParams(request.Params)
	}

	if request.Timeout != 0 {
		client.Timeout = request.Timeout
	}

	if request.Ja3 != "" {
		client.SetJA3(request.Ja3)
	}

	executor.URL = request.URL

	executor.Method = strings.ToUpper(request.Method)
	if request.Method == "" {
		executor.Method = nhttp.MethodGet
	}

	if checkMethods(request.Method) && request.Body != "" {
		executor.SetBody(request.Body)
	}

	if checkMethods(request.Method) && request.Multipart != nil {
		executor.SetMultipartFormData(request.Multipart)
	}

	if checkMethods(request.Method) && request.Json != nil {
		executor.SetJsonData(request.Json)
	}

	if checkMethods(request.Method) && request.Form != nil {
		executor.SetFormData(request.Form)
	}

	response, err := executor.Send()
	if err != nil {
		nhttp.Error(w, err.Error(), nhttp.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	err = json.NewEncoder(w).Encode(&response)
	if err != nil {
		nhttp.Error(w, err.Error(), nhttp.StatusBadRequest)
		return
	}
}

func setProxy(client *tlsHttpClient.Client, url string) error {
	proxy := tlsHttpClient.StringToProxy(url, "http")
	if proxy == nil {
		return errors.New("proxy is nil")
	}

	if err := client.SetProxy(proxy); err != nil {
		return err
	}

	return nil
}

func checkMethods(method string) bool {
	return method == nhttp.MethodPost || method == nhttp.MethodPut || method == nhttp.MethodPatch
}
