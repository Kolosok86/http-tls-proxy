package main

import (
	"flag"
	"log"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/Kolosok86/http"
	"github.com/kolosok86/proxy/internal/app"
	"github.com/kolosok86/proxy/internal/core"
	tls "github.com/refraction-networking/utls"
)

const usageMsg = "http tls proxy service address"

func main() {
	var addr *string
	if port, exists := os.LookupEnv("PORT"); exists {
		addr = flag.String("addr", ":"+port, usageMsg)
	} else {
		addr = flag.String("addr", ":3128", usageMsg)
	}

	logWriter := core.NewLogWriter(os.Stderr)
	defer logWriter.Close()

	logger := core.NewCondLogger(log.New(logWriter, "[PROXY] ", log.LstdFlags|log.Lshortfile), 20)

	server := http.Server{
		Addr:              *addr,
		Handler:           app.NewProxyHandler(10*time.Second, logger),
		ErrorLog:          log.New(logWriter, "[HTTP] ", log.LstdFlags|log.Lshortfile),
		TLSNextProto:      make(map[string]func(*http.Server, *tls.Conn, http.Handler)),
		ReadTimeout:       0,
		ReadHeaderTimeout: 0,
		WriteTimeout:      0,
		IdleTimeout:       0,
	}

	runtime.GOMAXPROCS(runtime.NumCPU())

	logger.Info("Server started and listening on port %s", strings.Replace(*addr, ":", "", 1))

	err := server.ListenAndServe()
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
