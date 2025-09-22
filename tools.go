package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type GetStarsParams struct {
	Username string `json:"username,omitempty" jsonschema:"Optional username for which to get the starred repositories or empty string to use the authenticated user"`
	Page     int    `json:"page,omitempty" jsonschema:"Optional page number from where to get results.Set to 0 to fetch all repositories"`
	Limit    int    `json:"limit,omitempty" jsonschema:"Optional limit of respositores to fetch.Set to 0 to fetch all repositories"`
}

type AddStarParams struct {
	Fullnames []string `json:"fullnames,omitempty" jsonschema:"Required list of repository full names that will be starred.A repository fullname is in the form of 'onwer/name'"`
}

type RemoveStarParams struct {
	Fullnames []string `json:"fullnames,omitempty" jsonschema:"Required list of repository full names that will be unstarred.A repository fullname is in the form of 'onwer/name'"`
}

func newToolError(message string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: fmt.Sprintf("error: %s", message)},
		},
	}
}

func GetStars(ctx context.Context, req *mcp.CallToolRequest, params *GetStarsParams) (*mcp.CallToolResult, any, error) {
	repos, err := ghFetchStarred(ctx, params.Username, params.Page, params.Limit)
	if err != nil {
		return newToolError("fetching starred repositories"), nil, nil
	}

	json, err := json.Marshal(repos)
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
			&mcp.TextContent{Text: string(json)},
			// &mcp.TextContent{Text: buf.String()},
		},
		// StructuredContent: repos,
	}, nil, nil
}

func AddStar(ctx context.Context, req *mcp.CallToolRequest, params *AddStarParams) (*mcp.CallToolResult, any, error) {
	for _, fullname := range params.Fullnames {
		parts := strings.SplitN(fullname, "/", 2)
		if err := ghSetStarred(ctx, parts[0], parts[1]); err != nil {
			return newToolError(fmt.Sprintf("star repository %s", fullname)), nil, nil
		}
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: fmt.Sprintf("successfully starred %d repositories", len(params.Fullnames))},
		},
	}, nil, nil
}

func RemoveStar(ctx context.Context, req *mcp.CallToolRequest, params *AddStarParams) (*mcp.CallToolResult, any, error) {
	for _, fullname := range params.Fullnames {
		parts := strings.SplitN(fullname, "/", 2)
		if err := ghSetUnstarred(ctx, parts[0], parts[1]); err != nil {
			return newToolError(fmt.Sprintf("unstar repository %s", fullname)), nil, nil
		}
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: fmt.Sprintf("successfully unstarred %d repositories", len(params.Fullnames))},
		},
	}, nil, nil
}
