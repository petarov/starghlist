package main

// gh_gql.go talks to GitHub's GraphQL API for star-list operations that the
// REST API doesn't expose (see /Users/ppetrov/.claude/plans/adaptive-napping-pike.md).
// It is self-contained and touches nothing else in the package. Entry points
// are called manually from main.go for now, pending Step 2 of the plan
// (wiring them up as real MCP tools in tools.go).

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

const ghGraphQLURL = "https://api.github.com/graphql"

var ghGQLHTTPClient = &http.Client{Timeout: 15 * time.Second}

type ghGQLError struct {
	Message string `json:"message"`
}

type ghGQLResponse struct {
	Data   json.RawMessage `json:"data"`
	Errors []ghGQLError    `json:"errors"`
}

type ghPageInfo struct {
	HasNextPage bool   `json:"hasNextPage"`
	EndCursor   string `json:"endCursor"`
}

type ghListSummary struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Slug      string `json:"slug"`
	IsPrivate bool   `json:"isPrivate"`
	Items     struct {
		TotalCount int `json:"totalCount"`
	} `json:"items"`
}

// gqlQuery POSTs a GraphQL query/mutation and decodes its "data" into out.
// GitHub's GraphQL API returns HTTP 200 even on failure, with an "errors"
// array instead — that array is checked before out is ever touched.
func gqlQuery(ctx context.Context, query string, vars map[string]any, out any) error {
	token := os.Getenv("GITHUB_TOKEN")
	if len(token) == 0 {
		return fmt.Errorf("GITHUB_TOKEN environment variable not set")
	}

	body, err := json.Marshal(map[string]any{
		"query":     query,
		"variables": vars,
	})
	if err != nil {
		return fmt.Errorf("marshal graphql request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, ghGraphQLURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build graphql request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "bearer "+token)

	res, err := ghGQLHTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("graphql request: %w", err)
	}
	defer res.Body.Close()

	raw, err := io.ReadAll(res.Body)
	if err != nil {
		return fmt.Errorf("read graphql response: %w", err)
	}

	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("graphql http %d: %s", res.StatusCode, string(raw))
	}

	var gr ghGQLResponse
	if err := json.Unmarshal(raw, &gr); err != nil {
		return fmt.Errorf("unmarshal graphql response: %w (body: %s)", err, string(raw))
	}

	if len(gr.Errors) > 0 {
		msgs := make([]string, len(gr.Errors))
		for i, e := range gr.Errors {
			msgs[i] = e.Message
		}
		return fmt.Errorf("graphql errors: %s", strings.Join(msgs, "; "))
	}

	if out != nil {
		if err := json.Unmarshal(gr.Data, out); err != nil {
			return fmt.Errorf("unmarshal graphql data: %w", err)
		}
	}

	return nil
}

// gqlRepoID resolves an "owner/repo" pair to a Repository node ID, the form
// updateUserListsForItem's itemId requires — not "owner/repo".
func gqlRepoID(ctx context.Context, owner, name string) (string, error) {
	const query = `
	query($owner: String!, $name: String!) {
	  repository(owner: $owner, name: $name) {
	    id
	    nameWithOwner
	  }
	}`

	var resp struct {
		Repository *struct {
			ID            string `json:"id"`
			NameWithOwner string `json:"nameWithOwner"`
		} `json:"repository"`
	}

	if err := gqlQuery(ctx, query, map[string]any{"owner": owner, "name": name}, &resp); err != nil {
		return "", fmt.Errorf("resolve repo id for %s/%s: %w", owner, name, err)
	}
	if resp.Repository == nil {
		return "", fmt.Errorf("repository %s/%s not found", owner, name)
	}

	return resp.Repository.ID, nil
}

// gqlAllLists fetches the viewer's lists (id/name/slug/isPrivate). It does not
// page past 100 lists — this account has 13, and a second page would need its
// own cursor loop.
func gqlAllLists(ctx context.Context) ([]ghListSummary, error) {
	const query = `
	query {
	  viewer {
	    lists(first: 100) {
	      pageInfo { hasNextPage }
	      nodes {
	        id
	        name
	        slug
	        isPrivate
	        items(first: 1) { totalCount }
	      }
	    }
	  }
	}`

	var resp struct {
		Viewer struct {
			Lists struct {
				PageInfo ghPageInfo      `json:"pageInfo"`
				Nodes    []ghListSummary `json:"nodes"`
			} `json:"lists"`
		} `json:"viewer"`
	}

	if err := gqlQuery(ctx, query, nil, &resp); err != nil {
		return nil, fmt.Errorf("fetch lists: %w", err)
	}
	if resp.Viewer.Lists.PageInfo.HasNextPage {
		return nil, fmt.Errorf("more than 100 lists exist; pagination not implemented")
	}

	return resp.Viewer.Lists.Nodes, nil
}

// gqlListContainsRepo pages through a single list's items via node(id:) —
// there is no User.list(slug:) field, so this is the only way to look up one
// list's contents by ID. Pagination matters here: at least one existing list
// already has 93 items.
func gqlListContainsRepo(ctx context.Context, listID, repoID string) (bool, error) {
	const query = `
	query($id: ID!, $after: String) {
	  node(id: $id) {
	    ... on UserList {
	      items(first: 100, after: $after) {
	        pageInfo { hasNextPage endCursor }
	        nodes {
	          __typename
	          ... on Repository { id }
	        }
	      }
	    }
	  }
	}`

	var after string
	for {
		vars := map[string]any{"id": listID}
		if len(after) > 0 {
			vars["after"] = after
		}

		var resp struct {
			Node struct {
				Items struct {
					PageInfo ghPageInfo `json:"pageInfo"`
					Nodes    []struct {
						Typename string `json:"__typename"`
						ID       string `json:"id"`
					} `json:"nodes"`
				} `json:"items"`
			} `json:"node"`
		}

		if err := gqlQuery(ctx, query, vars, &resp); err != nil {
			return false, fmt.Errorf("page list %s: %w", listID, err)
		}

		for _, n := range resp.Node.Items.Nodes {
			// nodes is [UserListItems] with nullable elements per the schema;
			// an empty ID here means a null slot, not a match.
			if len(n.ID) > 0 && n.ID == repoID {
				return true, nil
			}
		}

		if !resp.Node.Items.PageInfo.HasNextPage {
			return false, nil
		}
		after = resp.Node.Items.PageInfo.EndCursor
	}
}

// gqlMembershipConcurrent runs gqlListContainsRepo for every list in
// parallel, capped at ghMembershipConcurrency simultaneous requests, and
// returns the IDs of lists that contain repoID.
//
// This exists to cut wall-clock time versus checking each list one after
// another, WITHOUT changing the query shape: each request is still the exact
// single-list node(id:) query already proven not to trigger the 502 that a
// combined/nested/aliased-batch query did. Concurrency only changes when
// those identical requests are sent, not what's asked for in any one of them.
func gqlMembershipConcurrent(ctx context.Context, lists []ghListSummary, repoID string) ([]string, error) {
	const ghMembershipConcurrency = 5

	type result struct {
		listID string
		found  bool
		err    error
	}

	results := make(chan result, len(lists))
	sem := make(chan struct{}, ghMembershipConcurrency)

	var wg sync.WaitGroup
	for _, l := range lists {
		wg.Add(1)
		go func(l ghListSummary) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			found, err := gqlListContainsRepo(ctx, l.ID, repoID)
			results <- result{listID: l.ID, found: found, err: err}
		}(l)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	var memberListIDs []string
	var firstErr error
	for r := range results {
		if r.err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("check membership in list %s: %w", r.listID, r.err)
			}
			continue
		}
		if r.found {
			memberListIDs = append(memberListIDs, r.listID)
		}
	}

	return memberListIDs, firstErr
}

// gqlUpdateListsForItem calls updateUserListsForItem. listIds is the
// COMPLETE desired membership set, not a delta.
func gqlUpdateListsForItem(ctx context.Context, repoID string, listIDs []string) ([]ghListSummary, error) {
	const query = `
	mutation($itemId: ID!, $listIds: [ID!]!) {
	  updateUserListsForItem(input: {itemId: $itemId, listIds: $listIds}) {
	    lists {
	      id
	      name
	    }
	  }
	}`

	var resp struct {
		UpdateUserListsForItem struct {
			Lists []ghListSummary `json:"lists"`
		} `json:"updateUserListsForItem"`
	}

	vars := map[string]any{
		"itemId":  repoID,
		"listIds": listIDs,
	}

	if err := gqlQuery(ctx, query, vars, &resp); err != nil {
		return nil, fmt.Errorf("update lists for item: %w", err)
	}

	return resp.UpdateUserListsForItem.Lists, nil
}

// findList matches ref against a list's ID, then slug, then name — in that
// priority order, since slug/name collisions across those fields are
// possible in principle (e.g. a list literally named after another list's
// slug) and ID is unambiguous by construction.
func findList(lists []ghListSummary, ref string) *ghListSummary {
	for i := range lists {
		if lists[i].ID == ref {
			return &lists[i]
		}
	}
	for i := range lists {
		if strings.EqualFold(lists[i].Slug, ref) {
			return &lists[i]
		}
	}
	for i := range lists {
		if strings.EqualFold(lists[i].Name, ref) {
			return &lists[i]
		}
	}
	return nil
}

// ghListStarLists returns every star list on the authenticated account —
// name, node ID, slug, visibility, and item count for each. Use the
// returned ID/Slug/Name as ghAddRepoToList / ghRemoveRepoFromList's listRef
// argument.
func ghListStarLists(ctx context.Context) ([]ghListSummary, error) {
	return gqlAllLists(ctx)
}

// ghSetListMembership is the shared read-modify-write behind
// ghAddRepoToList and ghRemoveRepoFromList. GraphQL's listIds argument is a
// COMPLETE replacement, not a delta, so both directions need the same steps
// — resolve repo + list, read current membership, compute the new complete
// set, mutate — and only differ in whether the target list ID is added to or
// removed from that set. Kept as one function so shared logic lives once;
// the two public entry points stay separate and self-describing rather than
// taking an ambiguous bool, matching how AddStar/RemoveStar are already
// split in this codebase.
func ghSetListMembership(ctx context.Context, fullname, listRef string, add bool) error {
	owner, name, err := parseRepoName(fullname)
	if err != nil {
		return err
	}

	lists, err := gqlAllLists(ctx)
	if err != nil {
		return err
	}

	target := findList(lists, listRef)
	if target == nil {
		refs := make([]string, len(lists))
		for i, l := range lists {
			refs[i] = fmt.Sprintf("%s (slug=%s)", l.Name, l.Slug)
		}
		sort.Strings(refs)
		return fmt.Errorf("no list matching %q by id/slug/name; available lists: %s", listRef, strings.Join(refs, ", "))
	}

	repoID, err := gqlRepoID(ctx, owner, name)
	if err != nil {
		return err
	}
	log.Printf("resolved repo   %s -> %s", fullname, repoID)
	log.Printf("resolved list   %q -> %s", target.Name, target.ID)

	currentListIDs, err := gqlMembershipConcurrent(ctx, lists, repoID)
	if err != nil {
		return err
	}
	log.Printf("current membership: %d list(s)", len(currentListIDs))

	isMember := false
	for _, id := range currentListIDs {
		if id == target.ID {
			isMember = true
			break
		}
	}

	var newListIDs []string
	switch {
	case add && isMember:
		log.Printf("%s is already in list %q — no mutation issued", fullname, target.Name)
		return nil
	case add:
		newListIDs = append(append([]string{}, currentListIDs...), target.ID)
	case !add && !isMember:
		log.Printf("%s is not in list %q — no mutation issued", fullname, target.Name)
		return nil
	default: // remove, and it is a member
		for _, id := range currentListIDs {
			if id != target.ID {
				newListIDs = append(newListIDs, id)
			}
		}
		// newListIDs may end up empty here — this is the live test of
		// whether the mutation accepts listIds: [] when the target list
		// was the repo's only membership.
	}

	result, err := gqlUpdateListsForItem(ctx, repoID, newListIDs)
	if err != nil {
		return err
	}

	log.Printf("%s now belongs to %d list(s):", fullname, len(result))
	for _, l := range result {
		log.Printf("  - %s", l.Name)
	}
	if len(result) == 0 {
		log.Print("  (none)")
	}

	return nil
}

// ghAddRepoToList adds fullname ("owner/repo") to the star list identified
// by listRef, then logs the repo's resulting list membership. listRef is
// matched, in order, against each list's node ID, slug, then name (all
// case-insensitive except ID which is case-sensitive by construction) — so
// any of the three fields ghListStarLists returns works as-is, no need to
// type the emoji-laden display name.
//
// It is read-modify-write and deliberately non-destructive: it reads the
// repo's current memberships across every list, unions in the target list,
// and writes the whole set back — existing memberships are preserved. If the
// repo is already in the target list, it reports that and returns without
// issuing the mutation.
func ghAddRepoToList(ctx context.Context, fullname, listRef string) error {
	return ghSetListMembership(ctx, fullname, listRef, true)
}

// ghRemoveRepoFromList removes fullname ("owner/repo") from the star list
// identified by listRef (same id/slug/name matching as ghAddRepoToList),
// then logs the repo's resulting list membership.
//
// It is read-modify-write: it reads the repo's current memberships, drops
// the target list from that set, and writes the remainder back — every
// other membership is preserved. If the repo isn't in the target list, it
// reports that and returns without issuing the mutation. If the target list
// was the repo's ONLY membership, this sends listIds: [].
func ghRemoveRepoFromList(ctx context.Context, fullname, listRef string) error {
	return ghSetListMembership(ctx, fullname, listRef, false)
}
