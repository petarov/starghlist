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
	defaultListenPort = 24240
)

var (
	version = "?.?"
	host    = flag.String("host", "localhost", "host to connect to/listen on")
	port    = flag.Int("port", defaultListenPort, "port number to connect to/listen on")
)

func main() {
	out := flag.CommandLine.Output()
	flag.Usage = func() {
		fmt.Fprintf(out, "%s %s\n", appName, version)
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
	initHttpClient()
	initGithubRestClient(ctx, token)
	initGithubGraphQLClient(token)

	server := mcp.NewServer(&mcp.Implementation{
		Name:    "starghlist-server",
		Version: version,
	}, nil)

	mcp.AddTool(server, &mcp.Tool{
		Name: "getStars",
		Description: "List starred GitHub repositories. " +
			"With no username: the authenticated user's own stars. With a username: that user's " +
			"PUBLIC stars instead — useful for browsing someone else's starred repositories, but " +
			"those results cannot be passed to addStar or removeStar, which only ever act on the " +
			"authenticated user's own stars. " +
			"Returns repository identifiers in \"owner/repo\" form along with metadata " +
			"(description, primary language, star count, topics). " +
			"Results are paginated via the page and limit parameters (see their own descriptions). " +
			"HOW TO CHOOSE limit AND page: omit limit entirely unless the user named a " +
			"specific number. Omitting it returns the 25 most recently starred repositories " +
			"(page 1), the correct answer for open-ended requests. Examples: \"show me my " +
			"starred repositories\" / \"what have I starred\" / \"list my stars\" -> omit " +
			"limit. \"show me 10 of my stars\" / \"my last 5 stars\" -> limit: 10 / limit: 5. " +
			"If the user wants EVERYTHING (\"all of them\", \"every starred repo\", \"the " +
			"complete list\", \"fetch all\", \"yes, fetch all\") — by ITSELF this NEVER means " +
			"limit: -1. It means: call this tool repeatedly, page: 1 then page: 2, page: 3, " +
			"… at the default (or a stated) page size, stopping once a call returns fewer " +
			"repositories than that page size (that was the last page). Do this even though " +
			"it takes several tool calls — that is the entire point, since it keeps every " +
			"individual response small enough to process regardless of how many repos exist. " +
			"limit: -1 returns the WHOLE result set as ONE unbounded response with no size " +
			"cap, which can be too large to process or fail outright. Only use limit: -1 if " +
			"the user explicitly rules out multiple calls in their own words, e.g. \"in a " +
			"single response, don't paginate\" or \"one shot, no separate calls\" — plain " +
			"\"all\"/\"everything\"/\"fetch all\" phrasing does NOT qualify on its own and " +
			"still means: iterate pages. " +
			"Use this to answer questions about what a user has starred, to search or filter " +
			"stars by topic or language, or to resolve a vague repo reference " +
			"(\"that Golang TUI library I starred\") into an exact \"owner/repo\" name before " +
			"calling addStar or removeStar. " +
			"This returns individual repositories, not star lists — for list names use getStarListNames.",
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
		Description: "List GitHub star lists — the user-created categories (such as \"Dev Tools\" or " +
			"\"Machine Learning\") into which starred repositories are organized. Not the repositories " +
			"inside them. " +
			"With no username: the authenticated user's own lists, each with name, a stable slug " +
			"identifier, visibility, and item count — use this to find the correct identifier before " +
			"calling addStarToList or removeStarFromList, preferring the slug over the name since list " +
			"names may contain emoji. " +
			"With a username: that user's PUBLIC lists as name+slug pairs (no id, visibility, or item " +
			"count). These belong to a different user and cannot be passed to addStarToList or " +
			"removeStarFromList, which only ever act on the authenticated user's own lists. " +
			"A starred repository may belong to several lists or to none.",
	}, GetLists)

	mcp.AddTool(server, &mcp.Tool{
		Name: "addStarToList",
		Description: "Add one or more of the authenticated user's starred GitHub repositories to one " +
			"of their star lists. Each repository must be given as \"owner/repo\"; the repository " +
			"should typically already be starred — call addStar first if unsure. " +
			"The listName argument is matched against a list's slug, name, or ID; use " +
			"getStarListNames to find a valid identifier, preferring the slug since list names may " +
			"contain emoji. This modifies the user's account. Adding a repository already in the list " +
			"is a no-op and is not an error, so it can be added without checking membership first.",
	}, AddStarToList)

	mcp.AddTool(server, &mcp.Tool{
		Name: "removeStarFromList",
		Description: "Remove one or more repositories from one of the authenticated user's star lists. " +
			"Each repository must be given as \"owner/repo\". " +
			"The listName argument is matched against a list's slug, name, or ID; use " +
			"getStarListNames to find a valid identifier, preferring the slug since list names may " +
			"contain emoji. This modifies the user's account and is the inverse of addStarToList. " +
			"It does not unstar the repository or affect its membership in any other list — the " +
			"repository stays starred and remains in every other list it belongs to. Removing a " +
			"repository that isn't in the list is a no-op and is not an error.",
	}, RemoveStarFromList)

	handler := mcp.NewStreamableHTTPHandler(func(request *http.Request) *mcp.Server {
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
	log.Printf("%s v%s MCP server listening on %s", appName, version, listenAddress)

	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("Error starting server: %v", err)
	}
}
