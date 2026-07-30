package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// defaultStarsLimit caps an unspecified getStars call, since fetching every
// starred repository can easily overflow a small-context local LLM.
const defaultStarsLimit = 25

type GetStarsParams struct {
	Username string `json:"username,omitempty" jsonschema:"Optional username for which to get the starred repositories or empty string to use the authenticated user"`
	Page     int    `json:"page,omitempty" jsonschema:"Optional page number to fetch, using the page size given by limit (or its default of 25). Only meaningful together with a positive limit; ignored when limit is -1, which always returns everything"`
	Limit    int    `json:"limit,omitempty" jsonschema:"Optional maximum number of repositories to return. Defaults to 25 when omitted. Set to -1 to fetch every starred repository with no limit - this can be slow and return a very large result for a user with many stars"`
}

type AddStarParams struct {
	Fullnames []string `json:"fullnames,omitempty" jsonschema:"Required list of repository full names that will be starred.A repository fullname is in the form of 'onwer/name'"`
}

type RemoveStarParams struct {
	Fullnames []string `json:"fullnames,omitempty" jsonschema:"Required list of repository full names that will be unstarred.A repository fullname is in the form of 'onwer/name'"`
}

type GetListsParams struct {
	Username string `json:"username,omitempty" jsonschema:"Optional username to get another user's public star list names (returns plain name strings only, no id/slug). Omit to get the authenticated user's own lists (richer output - name, slug, id, visibility, item count). Names returned for another user are NOT usable as the listName argument to addStarToList/removeStarFromList, which only operate on the authenticated user's own lists"`
}

type AddStarToListParams struct {
	Fullnames []string `json:"fullnames,omitempty" jsonschema:"Required list of repository full names to add to the list. A repository fullname is in the form of 'owner/name'"`
	ListName  string   `json:"listName,omitempty" jsonschema:"Required identifier of the star list to add the repositories to - a list's slug, name, or ID as returned by getStarListNames"`
}

type RemoveStarFromListParams struct {
	Fullnames []string `json:"fullnames,omitempty" jsonschema:"Required list of repository full names to remove from the list. A repository fullname is in the form of 'owner/name'"`
	ListName  string   `json:"listName,omitempty" jsonschema:"Required identifier of the star list to remove the repositories from - a list's slug, name, or ID as returned by getStarListNames"`
}

func newToolError(message string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: fmt.Sprintf("error: %s", message)},
		},
	}
}

func GetStars(ctx context.Context, req *mcp.CallToolRequest, params *GetStarsParams) (*mcp.CallToolResult, any, error) {
	limit := params.Limit
	switch {
	case limit == 0:
		limit = defaultStarsLimit
	case limit < 0:
		limit = 0 // no limit
	}

	repos, err := ghFetchStarred(ctx, params.Username, params.Page, limit)
	if err != nil {
		return newToolError("fetching starred repositories"), nil, nil
	}

	data, err := json.Marshal(repos)
	if err != nil {
		return newToolError("json marshal repositories"), nil, nil
	}

	// var buf bytes.Buffer
	// writer := csv.NewWriter(&buf)

	// writer.Write([]string{
	// 	"StarredAt", "ID", "OwnerName", "Name", "FullName", "Description",
	// 	"CreatedAt", "UpdatedAt", "Language", "Fork", "ForksCount",
	// 	"StargazersCount", "LicenseName", "HTMLURL",
	// })

	// for _, r := range repos {
	// 	record := []string{
	// 		r.StarredAt.String(),
	// 		strconv.FormatInt(r.ID, 10),
	// 		r.OwnerName,
	// 		r.Name,
	// 		r.FullName,
	// 		r.Description,
	// 		r.CreatedAt.String(),
	// 		r.UpdatedAt.String(),
	// 		r.Language,
	// 		strconv.FormatBool(r.Fork),
	// 		strconv.Itoa(r.ForksCount),
	// 		strconv.Itoa(r.StargazersCount),
	// 		r.LicenseName,
	// 		r.HTMLURL,
	// 	}
	// 	writer.Write(record)
	// }

	// writer.Flush()
	// if err := writer.Error(); err != nil {
	// 	return newToolError("writing csv results"), nil, nil
	// }

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: string(data)},
			// &mcp.TextContent{Text: buf.String()},
		},
		// StructuredContent: repos,
	}, nil, nil
}

func AddStar(ctx context.Context, req *mcp.CallToolRequest, params *AddStarParams) (*mcp.CallToolResult, any, error) {
	for _, fullname := range params.Fullnames {
		owner, name, err := parseRepoName(fullname)
		if err != nil {
			return newToolError(err.Error()), nil, nil
		}
		if err := ghSetStarred(ctx, owner, name); err != nil {
			return newToolError(fmt.Sprintf("star repository: %s", fullname)), nil, nil
		}
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: fmt.Sprintf("successfully starred %d repositories", len(params.Fullnames))},
		},
	}, nil, nil
}

func RemoveStar(ctx context.Context, req *mcp.CallToolRequest,
	params *RemoveStarParams) (*mcp.CallToolResult, any, error) {
	for _, fullname := range params.Fullnames {
		owner, name, err := parseRepoName(fullname)
		if err != nil {
			return newToolError(err.Error()), nil, nil
		}
		if err := ghSetUnstarred(ctx, owner, name); err != nil {
			return newToolError(fmt.Sprintf("unstar repository: %s", fullname)), nil, nil
		}
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: fmt.Sprintf("successfully unstarred %d repositories", len(params.Fullnames))},
		},
	}, nil, nil
}

func GetLists(ctx context.Context, req *mcp.CallToolRequest, params *GetListsParams) (*mcp.CallToolResult, any, error) {
	var data []byte

	if len(params.Username) > 0 {
		// Another user's lists aren't reachable via GraphQL (viewer-only), so
		// fall back to the HTML scrape
		names, err := ghGetLists(ctx, params.Username)
		if err != nil {
			return newToolError("fetching lists"), nil, nil
		}

		data, err = json.Marshal(names)
		if err != nil {
			return newToolError("json marshal lists"), nil, nil
		}
	} else {
		lists, err := ghListStarLists(ctx)
		if err != nil {
			return newToolError("fetching lists"), nil, nil
		}

		data, err = json.Marshal(lists)
		if err != nil {
			return newToolError("json marshal lists"), nil, nil
		}
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: string(data)},
		},
	}, nil, nil
}

func AddStarToList(ctx context.Context, req *mcp.CallToolRequest, params *AddStarToListParams) (*mcp.CallToolResult, any, error) {
	for _, fullname := range params.Fullnames {
		if err := ghAddRepoToList(ctx, fullname, params.ListName); err != nil {
			return newToolError(err.Error()), nil, nil
		}
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: fmt.Sprintf("successfully added %d repositories to list %q", len(params.Fullnames), params.ListName)},
		},
	}, nil, nil
}

func RemoveStarFromList(ctx context.Context, req *mcp.CallToolRequest, params *RemoveStarFromListParams) (*mcp.CallToolResult, any, error) {
	for _, fullname := range params.Fullnames {
		if err := ghRemoveRepoFromList(ctx, fullname, params.ListName); err != nil {
			return newToolError(err.Error()), nil, nil
		}
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: fmt.Sprintf("successfully removed %d repositories from list %q", len(params.Fullnames), params.ListName)},
		},
	}, nil, nil
}
