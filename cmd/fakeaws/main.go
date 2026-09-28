// Command fakeaws runs a local fake of the AWS APIs PickRole uses, for
// testing the app without an AWS account. See internal/fakeaws and
// scripts/mock-aws.ps1.
package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/pickrole/pickrole/internal/fakeaws"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:4599", "address to listen on")
	autoApprove := flag.Bool("auto-approve", false, "approve sign-ins without the browser page")
	credsTTL := flag.Duration("creds-ttl", time.Hour, "how long role credentials last; short values show automatic renewal")
	allowRemote := flag.Bool("allow-remote", false, "allow listening outside loopback (no authentication: anyone on the network can use it)")
	flag.Parse()

	// It has no authentication at all: keep it on this machine unless asked.
	if host, _, err := net.SplitHostPort(*addr); err != nil || !loopback(host) {
		if !*allowRemote {
			log.Fatalf("refusing to listen on %q: use a loopback address (127.0.0.1) or -allow-remote", *addr)
		}
	}

	srv := fakeaws.New(fakeaws.Options{AutoApprove: *autoApprove, CredentialsTTL: *credsTTL, Log: os.Stdout})
	base := "http://" + *addr
	fmt.Printf("PickRole fake AWS at %s\n", base)
	fmt.Printf("Point PickRole at it with AWS_ENDPOINT_URL=%s\n", base)
	fmt.Printf("Dashboard (accounts, expire the session): %s/\n\n", base)
	httpSrv := &http.Server{Addr: *addr, Handler: srv, ReadHeaderTimeout: 10 * time.Second}
	log.Fatal(httpSrv.ListenAndServe())
}

func loopback(host string) bool {
	ip := net.ParseIP(host)
	return host == "localhost" || (ip != nil && ip.IsLoopback())
}
