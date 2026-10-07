package main

import (
	"context"
	"log"

	"github.com/shibukawa/popcornweb/pw"
	"scrumify/handlers"
	"scrumify/pages"
	// Registers the engine the configured DSN names.
	_ "github.com/shibukawa/popcornweb/database/postgres"
	// session.backend = "rdb" is served by this import; storage is opt-in.
	_ "github.com/shibukawa/popcornweb/sessionstore/postgres"
	// The single-use login records this engine stores.
	_ "github.com/shibukawa/popcornweb/authstate/postgres"
	// Project timezones must resolve inside a container image without tzdata.
	_ "time/tzdata"
)

func main() {
	// Names the API document served at server.openapi_path and shown by the
	// reference UI at /docs. Without it both fall back to "Application API".
	if err := pw.SetOpenAPIInfo(pw.OpenAPIInfo{Title: "scrumify", Version: "0.1.0"}); err != nil {
		log.Fatal(err)
	}

	// Installed before Run: the framework calls these while it serves a login.
	handlers.RegisterAccounts()
	handlers.RegisterDemoData()
	// The page routes join the handler mux. Registration order does not
	// matter; a duplicate pattern would panic here rather than shadow.
	mux := handlers.Handlers()
	pages.Register(mux)
	if err := pw.Run(context.Background(), mux); err != nil {
		log.Fatal(err)
	}
}
