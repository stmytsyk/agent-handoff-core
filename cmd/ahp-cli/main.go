package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"time"
)

const socketPath = "/tmp/ahp.sock"

func main() {
	if len(os.Args) < 2 {
		log.Fatal("usage: ahp <share|ingest> [target] [--from @handle] [--peer @handle] [--relay http://host:port]")
	}
	command := os.Args[1]
	req := map[string]any{"command": command}

	switch command {
	case "init":
		fs := flag.NewFlagSet("init", flag.ExitOnError)
		handle := fs.String("handle", os.Getenv("AHP_HANDLE"), "local handle")
		relay := fs.String("relay", os.Getenv("AHP_RELAY_URL"), "relay URL")
		if err := fs.Parse(os.Args[2:]); err != nil {
			log.Fatal(err)
		}
		req["local_handle"] = *handle
		req["relay_url"] = *relay
	case "contact":
		if len(os.Args) < 3 {
			log.Fatal("usage: ahp contact <show|add> [contact-string]")
		}
		req["command"] = "contact_" + os.Args[2]
		if os.Args[2] == "add" {
			fs := flag.NewFlagSet("contact add", flag.ExitOnError)
			trust := fs.Bool("trust", false, "mark contact trusted")
			if err := fs.Parse(os.Args[3:]); err != nil {
				log.Fatal(err)
			}
			if fs.NArg() < 1 {
				log.Fatal("usage: ahp contact add <ahp:...>")
			}
			req["contact_string"] = fs.Arg(0)
			req["trusted"] = *trust
		}
	case "share":
		fs := flag.NewFlagSet("share", flag.ExitOnError)
		from := fs.String("from", os.Getenv("AHP_HANDLE"), "local handle")
		relay := fs.String("relay", os.Getenv("AHP_RELAY_URL"), "relay URL")
		timeout := fs.Duration("timeout", 30*time.Second, "operation timeout")
		if err := fs.Parse(os.Args[2:]); err != nil {
			log.Fatal(err)
		}
		if fs.NArg() > 0 {
			req["target"] = fs.Arg(0)
		}
		req["local_handle"] = *from
		req["relay_url"] = *relay
		req["timeout_ms"] = timeout.Milliseconds()
	case "ingest":
		fs := flag.NewFlagSet("ingest", flag.ExitOnError)
		from := fs.String("from", os.Getenv("AHP_HANDLE"), "local handle")
		peer := fs.String("peer", "", "expected sender handle")
		relay := fs.String("relay", os.Getenv("AHP_RELAY_URL"), "relay URL")
		timeout := fs.Duration("timeout", 60*time.Second, "operation timeout")
		if err := fs.Parse(os.Args[2:]); err != nil {
			log.Fatal(err)
		}
		req["local_handle"] = *from
		req["target"] = *peer
		req["relay_url"] = *relay
		req["timeout_ms"] = timeout.Milliseconds()
	default:
		log.Fatalf("unknown command %q", command)
	}

	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	if err := json.NewEncoder(conn).Encode(req); err != nil {
		log.Fatal(err)
	}
	var resp map[string]any
	if err := json.NewDecoder(conn).Decode(&resp); err != nil {
		log.Fatal(err)
	}
	out, _ := json.MarshalIndent(resp, "", "  ")
	fmt.Println(string(out))
}
