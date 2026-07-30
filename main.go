package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
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
		Name: "getStars",
		Description: "List the GitHub repositories that the authenticated user has starred. " +
			"Returns repository identifiers in \"owner/repo\" form along with metadata " +
			"(description, primary language, star count, topics). " +
			"Use this to answer questions about what the user has starred, to search or filter " +
			"their stars by topic or language, or to resolve a vague repo reference " +
			"(\"that Rust TUI library I starred\") into an exact \"owner/repo\" name before " +
			"calling addStar or removeStar. " +
			"This returns individual repositories, not the user's star lists — for list names use getListNames.",
	}, GetStars)

	mcp.AddTool(server, &mcp.Tool{
		Name: "addStar",
		Description: "Star one or more GitHub repositories on behalf of the authenticated user. " +
			"Each repository must be given as \"owner/repo\" (for example \"golang/go\"); " +
			"bare names, URLs, or descriptions will not resolve — use getStars or ask the user " +
			"to disambiguate first. " +
			"This modifies the user's account. Starring an already-starred repository is a no-op " +
			"and is not an error, so a repository can be starred without checking its current state first.",
	}, AddStar)

	mcp.AddTool(server, &mcp.Tool{
		Name: "removeStar",
		Description: "Unstar (remove the star from) one or more GitHub repositories for the authenticated user. " +
			"Each repository must be given as \"owner/repo\" (for example \"golang/go\"). " +
			"This modifies the user's account and is the inverse of addStar. " +
			"Unstarring a repository the user has not starred is a no-op and is not an error. " +
			"This does not delete, archive, or otherwise affect the repository itself — it only " +
			"removes it from the user's list of stars.",
	}, RemoveStar)

	mcp.AddTool(server, &mcp.Tool{
		Name: "getStarListNames",
		Description: "List the names of the user's GitHub star lists — the user-created categories " +
			"(such as \"Dev Tools\" or \"Machine Learning\") into which their starred repositories " +
			"are organized. Returns list names only, not the repositories inside them. " +
			"Use this to discover which categories exist, or to find the correct list name before " +
			"referring to one. A starred repository may belong to several lists or to none.",
	}, GetLists)

	handler := mcp.NewSSEHandler(func(request *http.Request) *mcp.Server {
		logRequest(request)
		return server
	}, nil)

	listenAddress := fmt.Sprintf("%s:%d", *host, *port)
	srv := &http.Server{
		Addr:        listenAddress,
		Handler:     handler,
		ReadTimeout: 20 * time.Second,
		IdleTimeout: 120 * time.Second,
	}
	log.Printf("%s %s MCP server listening on %s", appName, appVersion,
		listenAddress)

	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("Error starting server: %v", err)
	}
}
