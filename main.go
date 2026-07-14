package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	appName           = "\u2606 Starghlist"
	appVersion        = "1.0.0"
	defaultListenPort = 24240
)

var (
	host = flag.String("host", "localhost", "host to connect to/listen on")
	port = flag.Int("port", defaultListenPort, "port number to connect to/listen on")
)

func main() {
	out := flag.CommandLine.Output()
	flag.Usage = func() {
		fmt.Fprintf(out, "%s %s\n", appName, appVersion)
		fmt.Fprintf(out, "Manage your GitHub starred repositories using MCP.\n\n")
		fmt.Fprintf(out, "Usage: %s [-port <port] [-host <host>]\n\n", os.Args[0])
		flag.PrintDefaults()
		os.Exit(1)
	}
	flag.Parse()

	token := os.Getenv("GITHUB_TOKEN")
	if len(token) == 0 {
		fmt.Fprint(out, "Missing `GITHUB_TOKEN` environment variable.")
		os.Exit(1)
	}

	logInit()

	ctx := context.Background()
	createGitHubClient(ctx, token)

	server := mcp.NewServer(&mcp.Implementation{
		Name:    "starghlist-server",
		Version: appVersion,
	}, nil)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "getStars",
		Description: "Get the users's starred repositories",
	}, GetStars)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "addStar",
		Description: "Add star to one or more repositories",
	}, AddStar)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "removeStar",
		Description: "Remove/unstar one or more repositories",
	}, RemoveStar)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "getListNames",
		Description: "Get all of the user's list names",
	}, GetLists)

	handler := mcp.NewSSEHandler(func(request *http.Request) *mcp.Server {
		logRequest(request)
		return server
	}, nil)

	listenAddress := fmt.Sprintf("%s:%d", *host, *port)
	log.Printf("%s %s MCP server listening on %s",
		appName, appVersion, listenAddress)

	srv := &http.Server{
		Addr:         listenAddress,
		Handler:      requireLocalOrigin(handler),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("Error starting server: %v", err)
	}
}

// requireLocalOrigin rejects browser requests whose Origin is not loopback,
// blocking DNS-rebinding / CSRF from a web page against this local server.
// Non-browser MCP clients send no Origin header and are unaffected.
func requireLocalOrigin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if o := r.Header.Get("Origin"); o != "" {
			u, err := url.Parse(o)
			if err != nil || (u.Hostname() != "localhost" &&
				u.Hostname() != "127.0.0.1" && u.Hostname() != "::1") {
				http.Error(w, "forbidden origin", http.StatusForbidden)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}
