package main

import "flag"

var (
	domain = flag.String("domain", "", "your ZITADEL instance domain (in the form: <instance>.zitadel.cloud or <yourdomain>)")
	key    = flag.String("key", "", "path to your api key.json")
	port   = flag.String("port", "8090", "port to run the server")
)
