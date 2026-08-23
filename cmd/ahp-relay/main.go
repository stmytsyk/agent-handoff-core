package main

import (
	"flag"
	"log"
	"net/http"

	"github.com/stmytsyk/agent-handoff-core/pkg/transport"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:8799", "HTTP listen address")
	flag.Parse()

	relay := transport.NewRelay()
	log.Printf("ahp relay listening on http://%s", *addr)
	log.Fatal(http.ListenAndServe(*addr, relay.Handler()))
}
