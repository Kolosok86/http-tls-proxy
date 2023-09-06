# Golang Http Proxy With TLS (JA3) and Header / Pseudo - Header support

# 🚀 Features

- Easy setup with default proxy agent
- Custom header order
- Custom pseudo-header order
- TLS (Ja3 Token) configuration
- Header order duplicated from your request

# How change tls or header order

### Add headers to your request 
- `proxy-tls` with ja3 token for change your tls 
- `proxy-protocol` with `http` or `https` parameter 
- `proxy-downgrade` use http/1.1 for request
- `proxy-tls-setup` set values to emulate as `android` `chrome` `ios` `firefox`
- `proxy-node-escape` remove header `Connection` from request

> default is chrome browser tls, https protocol and http2 / http

# How install

Clone repository

```bash
$ git clone https://github.com/Kolosok86/http-tls-proxy.git
```
Run with docker
```bash
$ make docker-build && make docker-run
```
Build and Run
```bash
$ make build && ./proxy
```

# How use

```js
import pkg from "https-proxy-agent"
import axios from "axios"

const { HttpsProxyAgent } = pkg

const ja3 =
  "771,4865-4866-4867-49195-49199-49196-49200-52393-52392-49171-49172-156-157-47-53,10-18-23-11-13-35-27-43-16-5-65281-45-17513-51-0-21,29-23-24,0"

const headers = {
  "User-Agent":
    "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/110.0.0.0 Safari/537.36",
  "Content-Type": "application/x-www-form-urlencoded",
  "proxy-node-escape": "true", // value is doesn't matter
  "proxy-tls": ja3,
}

const response = await axios.get("http://tls.peet.ws/api/all", {
  httpAgent: new HttpsProxyAgent("http://127.0.0.1:3128"),
  headers,
})

console.log(ja3 === response.data.tls.ja3)
```


> For change header order you can shuffle headers in your request

> I advise you to use the http(s) standard library, or the request library. It will be easier for you to set the headers you need there.

> Use http scheme in request url, proxy automatically change to https, if you want `http` set header `proxy-protocol`

> `proxy-tls` has priority `proxy-tls-setup`, set only one of this parameters