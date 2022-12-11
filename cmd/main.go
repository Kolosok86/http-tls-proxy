package main

import (
	"flag"
	"log"
	nhttp "net/http"
	"os"
	"runtime"

	"github.com/kolosok86/proxy/internal/app"
)

func main() {
	port, exists := os.LookupEnv("PORT")
	var addr *string
	if exists {
		addr = flag.String("addr", ":"+port, "http service address")
	} else {
		addr = flag.String("addr", ":5500", "http service address")
	}

	runtime.GOMAXPROCS(runtime.NumCPU())

	nhttp.HandleFunc("/api/request", app.HandleReq)

	err := nhttp.ListenAndServe(*addr, nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
