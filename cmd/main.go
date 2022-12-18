package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"runtime"
	"strings"

	"github.com/kolosok86/proxy/internal/app"
)

const usageMsg = "http tls proxy service address"

func main() {
	var addr *string
	if port, exists := os.LookupEnv("PORT"); exists {
		addr = flag.String("addr", ":"+port, usageMsg)
	} else {
		addr = flag.String("addr", ":4000", usageMsg)
	}

	runtime.GOMAXPROCS(runtime.NumCPU())
	http.HandleFunc("/api/request", app.HandleReq)

	fmt.Println("Server started and listening on port", strings.Replace(*addr, ":", "", 1))

	err := http.ListenAndServe(*addr, nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
