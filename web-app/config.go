package main

import (
	"embed"
	"flag"
)

var (
	// flags to be provided for running the example server
	domain      = flag.String("domain", "", "your ZITADEL instance domain (in the form: https://<instance>.zitadel.cloud or https://<yourdomain>)")
	key         = flag.String("key", "", "encryption key")
	clientID    = flag.String("clientID", "", "clientID provided by ZITADEL")
	redirectURI = flag.String("redirectURI", "", "redirectURI registered at ZITADEL")
	port        = flag.String("port", "8089", "port to run the server on (default is 8089)")

	//go:embed "templates/*.html"
	templates embed.FS
)
