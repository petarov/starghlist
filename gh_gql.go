package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"slices"
	"sort"
	"strings"
	"sync"

	"github.com/google/go-github/v74/github"
)

const (
	ghGraphQLURL                   = "https://api.github.com/graphql"
	ghGraphQLMembershipConcurrency = 5
	// ghGraphQLMaxPageSize is the largest page size the GitHub GraphQL API
	// allows for a connection's `first` argument
	ghGraphQLMaxPageSize = 100
)

var (
	gqlHttpAuthHeader = ""
)

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

func initGithubGraphQLClient(token string) {
	gqlHttpAuthHeader = "bearer " + token
}

// gqlQuery Internal. POSTs a GraphQL query/mutation and decodes its "data" into out.
func gqlQuery(ctx context.Context, query string, vars map[string]any, out any) error {
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
	req.Header.Set("Authorization", gqlHttpAuthHeader)

	res, err := globHttpClient.Do(req)
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

	var resp ghGQLResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return fmt.Errorf("unmarshal graphql response: %w (body: %s)", err, string(raw))
	}

	if len(resp.Errors) > 0 {
		msgs := make([]string, len(resp.Errors))
		for i, e := range resp.Errors {
			msgs[i] = e.Message
		}
		return fmt.Errorf("graphql errors: %s", strings.Join(msgs, "; "))
	}

	if out != nil {
		if err := json.Unmarshal(resp.Data, out); err != nil {
			return fmt.Errorf("unmarshal graphql data: %w", err)
		}
	}

	return nil
}

// gqlRepoID resolves an "owner/repo" pair to a Repository ID
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

// gqlAllLists fetches the viewer's lists (id/name/slug/isPrivate).
// It does not page past 100 lists!
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

// ghGQLRepoNode is the subset of Repository fields returned for a star list's
// items
//
// Nullable GraphQL fields (description, primaryLanguage, licenseInfo) decode
// into their zero value, which is what MetaRepo expects for a missing one.
type ghGQLRepoNode struct {
	NameWithOwner  string           `json:"nameWithOwner"`
	Name           string           `json:"name"`
	Description    string           `json:"description"`
	URL            string           `json:"url"`
	StargazerCount int              `json:"stargazerCount"`
	ForkCount      int              `json:"forkCount"`
	IsFork         bool             `json:"isFork"`
	CreatedAt      github.Timestamp `json:"createdAt"`
	UpdatedAt      github.Timestamp `json:"updatedAt"`
	Owner          struct {
		Login string `json:"login"`
	} `json:"owner"`
	PrimaryLanguage struct {
		Name string `json:"name"`
	} `json:"primaryLanguage"`
	LicenseInfo struct {
		Name string `json:"name"`
	} `json:"licenseInfo"`
}

// gqlListItemsQuery is a single page of a list's items. Lists have no
// User.list(slug:) field, so node(id:) is the only way in — same as
// gqlListContainsRepo, which asks for far fewer fields because it only needs
// repository IDs.
const gqlListItemsQuery = `
	query($id: ID!, $first: Int!, $after: String) {
	  node(id: $id) {
	    ... on UserList {
	      items(first: $first, after: $after) {
	        pageInfo { hasNextPage endCursor }
	        nodes {
	          __typename
	          ... on Repository {
	            nameWithOwner
	            name
	            description
	            url
	            stargazerCount
	            forkCount
	            isFork
	            owner { login }
	            primaryLanguage { name }
	            licenseInfo { name }
	            createdAt
	            updatedAt
	          }
	        }
	      }
	    }
	  }
	}`

// gqlListCountQuery counts the repositories in one page of a list's items
// without paying for the item payload. Node types are needed because the
// connection can hold null slots and non-repository items, which must not be
// counted as repositories.
const gqlListCountQuery = `
	query($id: ID!, $first: Int!, $after: String) {
	  node(id: $id) {
	    ... on UserList {
	      items(first: $first, after: $after) {
	        pageInfo { hasNextPage endCursor }
	        nodes { __typename }
	      }
	    }
	  }
	}`

type gqlListItemsResp struct {
	Node *struct {
		Items struct {
			PageInfo ghPageInfo `json:"pageInfo"`
			// nodes is [UserListItems] with nullable elements per the schema
			Nodes []struct {
				Typename string `json:"__typename"`
				ghGQLRepoNode
			} `json:"nodes"`
		} `json:"items"`
	} `json:"node"`
}

// gqlListItemsPage fetches one page (at most ghGraphQLMaxPageSize items) of the
// star list identified by listID, starting after the given cursor. An empty
// cursor starts at the beginning; the returned page info carries the next one.
func gqlListItemsPage(ctx context.Context, listID string, first int, after string) ([]MetaRepo, ghPageInfo, error) {
	vars := map[string]any{"id": listID, "first": first}
	if len(after) > 0 {
		vars["after"] = after
	}

	var resp gqlListItemsResp
	if err := gqlQuery(ctx, gqlListItemsQuery, vars, &resp); err != nil {
		return nil, ghPageInfo{}, fmt.Errorf("page items of list %s: %w", listID, err)
	}
	if resp.Node == nil {
		return nil, ghPageInfo{}, fmt.Errorf("list %s not found (deleted?) or not a star list", listID)
	}

	repos := make([]MetaRepo, 0, len(resp.Node.Items.Nodes))
	for _, n := range resp.Node.Items.Nodes {
		// only repositories count; null slots and other item types do not
		if n.Typename != "Repository" || len(n.NameWithOwner) == 0 {
			continue
		}
		repos = append(repos, newMetaRepoFromGQL(n.ghGQLRepoNode))
	}

	return repos, resp.Node.Items.PageInfo, nil
}

// gqlListCountPage is gqlListItemsPage without the item fields, used when paging
// past items that are not part of the requested result. It returns the number
// of repositories in the page, not the number of item slots, so that page
// boundaries are counted in repositories.
func gqlListCountPage(ctx context.Context, listID string, first int, after string) (int, ghPageInfo, error) {
	vars := map[string]any{"id": listID, "first": first}
	if len(after) > 0 {
		vars["after"] = after
	}

	var resp struct {
		Node *struct {
			Items struct {
				PageInfo ghPageInfo `json:"pageInfo"`
				Nodes    []struct {
					Typename string `json:"__typename"`
				} `json:"nodes"`
			} `json:"items"`
		} `json:"node"`
	}

	if err := gqlQuery(ctx, gqlListCountQuery, vars, &resp); err != nil {
		return 0, ghPageInfo{}, fmt.Errorf("count items of list %s: %w", listID, err)
	}
	if resp.Node == nil {
		return 0, ghPageInfo{}, fmt.Errorf("list %s not found (deleted?) or not a star list", listID)
	}

	repos := 0
	for _, n := range resp.Node.Items.Nodes {
		if n.Typename == "Repository" {
			repos++
		}
	}

	return repos, resp.Node.Items.PageInfo, nil
}

// ghFetchListRepos returns the repositories inside a star list.
//
// limit > 0 returns a single batch of at most limit repositories taken from the
// 1-based page (page size = limit), mirroring ghFetchStarred. limit <= 0 returns
// the whole list and ignores page, walking every cursor.
//
// The API paginates a list by cursor, so a page number is served by advancing
// (page-1)*limit repositories with count-only requests and fetching item fields
// only for the page that is actually returned. Page boundaries and batch sizes
// are counted in repositories, so a batch shorter than limit always means the
// list ended, never that a page happened to hold non-repository items.
func ghFetchListRepos(ctx context.Context, listID string, page, limit int) ([]MetaRepo, error) {
	if limit <= 0 {
		return gqlListAllRepos(ctx, listID)
	}
	if page < 1 {
		page = 1
	}

	after, ok, err := gqlListSkip(ctx, listID, (page-1)*limit)
	if err != nil {
		return nil, err
	}
	if !ok {
		// page is past the end of the list
		return []MetaRepo{}, nil
	}

	// fetch the page itself, in chunks of at most ghGraphQLMaxPageSize items;
	// a chunk holding null slots or non-repository items yields fewer than
	// chunk repositories, so keep going until limit is reached or the list ends
	repos := make([]MetaRepo, 0, limit)
	remaining := limit
	for remaining > 0 {
		chunk := min(remaining, ghGraphQLMaxPageSize)
		items, info, err := gqlListItemsPage(ctx, listID, chunk, after)
		if err != nil {
			return nil, err
		}
		repos = append(repos, items...)
		remaining -= len(items)

		next := info.EndCursor
		if remaining <= 0 || !info.HasNextPage || len(next) == 0 || next == after {
			break
		}
		after = next
	}

	return repos, nil
}

// gqlListSkip advances past the first count repositories of a list, without
// fetching their fields. It reports false when the list has fewer than count
// repositories.
func gqlListSkip(ctx context.Context, listID string, count int) (string, bool, error) {
	var after string

	for count > 0 {
		chunk := min(count, ghGraphQLMaxPageSize)
		repos, info, err := gqlListCountPage(ctx, listID, chunk, after)
		if err != nil {
			return "", false, err
		}
		if !info.HasNextPage {
			return "", false, nil
		}

		// a page may hold fewer repositories than slots; keep advancing
		// anyway, since the cursor still moves past all of the page's items
		next := info.EndCursor
		if len(next) == 0 || next == after {
			return "", false, nil
		}
		after = next
		count -= repos
	}

	return after, true, nil
}

// gqlListAllRepos walks a list's entire set of items
func gqlListAllRepos(ctx context.Context, listID string) ([]MetaRepo, error) {
	repos := []MetaRepo{}
	var after string

	for {
		items, info, err := gqlListItemsPage(ctx, listID, ghGraphQLMaxPageSize, after)
		if err != nil {
			return nil, err
		}
		repos = append(repos, items...)

		if !info.HasNextPage || len(info.EndCursor) == 0 || info.EndCursor == after {
			break
		}
		after = info.EndCursor
	}

	return repos, nil
}

// gqlListContainsRepo pages through a single list's items via node(id:) —
// there is no User.list(slug:) field, so this is the only way to look up one
// list's contents by ID.
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
			// nodes is [UserListItems] with nullable elements per the schema
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
// parallel, capped at ghGraphQLMembershipConcurrency simultaneous requests, and
// returns the IDs of lists that contain repoID.
//
// This exists to cut time as opposed to checking each list one after another.
func gqlMembershipConcurrent(ctx context.Context, lists []ghListSummary, repoID string) ([]string, error) {
	type result struct {
		listID string
		found  bool
		err    error
	}

	results := make(chan result, len(lists))
	semaphore := make(chan struct{}, ghGraphQLMembershipConcurrency)

	var wg sync.WaitGroup
	for _, entry := range lists {
		wg.Add(1)
		go func(list ghListSummary) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			found, err := gqlListContainsRepo(ctx, list.ID, repoID)
			results <- result{listID: list.ID, found: found, err: err}
		}(entry)
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

// listNotFoundError reports an unresolvable list reference and names the lists
// that were actually available, since the reference is usually a typo or a
// list name the agent invented
func listNotFoundError(lists []ghListSummary, ref string) error {
	if len(lists) == 0 {
		return fmt.Errorf("no list matching %q by id/slug/name; the account has no star lists", ref)
	}

	refs := make([]string, len(lists))
	for i, l := range lists {
		refs[i] = fmt.Sprintf("%s (slug=%s)", l.Name, l.Slug)
	}
	sort.Strings(refs)

	return fmt.Errorf("no list matching %q by id/slug/name; available lists: %s", ref, strings.Join(refs, ", "))
}

// findList matches ref against a list's ID, then slug, then name, in that order.
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

// ghListStarLists returns every star list on the authenticated account.
// Fields returned: name, node ID, slug, visibility, and item count for each.
func ghListStarLists(ctx context.Context) ([]ghListSummary, error) {
	return gqlAllLists(ctx)
}

// ghSetListMembership is the shared read-modify-write behind
// ghAddRepoToList and ghRemoveRepoFromList. GraphQL's listIds argument is a
// full replacement, not a delta, so both directions need the same steps:
// resolve repo + list, read current membership, compute the new complete
// set, mutate, and only differ in whether the target list ID is added to or
// removed from that set.
func ghSetListMembership(ctx context.Context, fullname, listRef string, add bool) error {
	verb := "remove"
	if add {
		verb = "add"
	}
	op := fmt.Sprintf("[%s %s -> %q]", verb, fullname, listRef)

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
		return listNotFoundError(lists, listRef)
	}

	repoID, err := gqlRepoID(ctx, owner, name)
	if err != nil {
		return err
	}

	currentListIDs, err := gqlMembershipConcurrent(ctx, lists, repoID)
	if err != nil {
		return err
	}

	// is the target list among the lists the curretnly contain the repo?
	isMember := slices.Contains(currentListIDs, target.ID)

	newListIDs := []string{}
	switch {
	case add && isMember:
		log.Printf("%s already a member — no mutation issued", op)
		return nil
	case add:
		newListIDs = append(append(newListIDs, currentListIDs...), target.ID)
	case !add && !isMember:
		log.Printf("%s not a member — no mutation issued", op)
		return nil
	default: // remove, and it is a member
		for _, id := range currentListIDs {
			if id != target.ID {
				newListIDs = append(newListIDs, id)
			}
		}
	}

	result, err := gqlUpdateListsForItem(ctx, repoID, newListIDs)
	if err != nil {
		return err
	}

	membership := "(none)"
	if len(result) > 0 {
		names := make([]string, len(result))
		for i, l := range result {
			names[i] = l.Name
		}
		membership = strings.Join(names, ", ")
	}
	log.Printf("%s done: %d -> %d list(s): %s", op, len(currentListIDs), len(result), membership)

	return nil
}

// ghAddRepoToList adds fullname ("owner/repo") to the star list identified
// by listRef. listRef is matched, in order, against each list's node ID, slug,
// then name (all case-insensitive except ID which is case-sensitive).
//
// It is read-modify-write: it reads the repo's current memberships across every
// list, unions in the target list, and writes the whole set back with existing
// memberships preserved. If the repo is already in the target list, it does not
// mutate the list.
func ghAddRepoToList(ctx context.Context, fullname, listRef string) error {
	return ghSetListMembership(ctx, fullname, listRef, true)
}

// ghRemoveRepoFromList removes fullname ("owner/repo") from the star list
// identified by listRef (same id/slug/name matching as ghAddRepoToList).
//
// It is read-modify-write: it reads the repo's current memberships, drops
// the target list from that set, and writes the remainder back with every
// other membership preserved. If the repo isn't in the target list, it
// reports that and returns without mutating the list.
func ghRemoveRepoFromList(ctx context.Context, fullname, listRef string) error {
	return ghSetListMembership(ctx, fullname, listRef, false)
}
