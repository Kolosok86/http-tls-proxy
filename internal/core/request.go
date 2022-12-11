package core

type RequestParams struct {
	Method          string            `json:"method"`
	URL             string            `json:"url"`
	Params          map[string]string `json:"params"`
	Headers         map[string]string `json:"headers"`
	Body            string            `json:"body"`
	Json            map[string]any    `json:"json"`
	Form            map[string]string `json:"form"`
	Multipart       map[string]string `json:"multipart"`
	Ja3             string            `json:"ja3"`
	UserAgent       string            `json:"userAgent"`
	Proxy           string            `json:"proxy"`
	Timeout         int               `json:"timeout"`
	DisableRedirect bool              `json:"disableRedirect"`
}
