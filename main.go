package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

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
		Name:        "getStarred",
		Description: "Get the users's starred repositories",
	}, GetStars)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "addStar",
		Description: "Add star or star one or more repositories",
	}, AddStar)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "removeStar",
		Description: "Remove star or unstar one or more repositories",
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

	if err := http.ListenAndServe(listenAddress, handler); err != nil {
		log.Fatalf("Error starting server: %v", err)
	}
}
