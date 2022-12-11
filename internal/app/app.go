package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/kolosok86/proxy/internal/core"
	"github.com/quotpw/tlsHttpClient/tlsHttpClient"
)

func HandleReq(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	request := &core.RequestParams{}
	err := json.NewDecoder(r.Body).Decode(request)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if request.URL == "" {
		http.Error(w, "Set url first", http.StatusBadRequest)
		return
	}

	fmt.Printf("Receive request to url %s:\n", request.URL)

	client := tlsHttpClient.New()
	executor := client.R()

	if request.Proxy != "" && setProxy(client, request.Proxy) != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
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
		executor.Method = http.MethodGet
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
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	err = json.NewEncoder(w).Encode(&response)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
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
	return method == http.MethodPost || method == http.MethodPut || method == http.MethodPatch
}
