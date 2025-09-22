package main

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/PuerkitoBio/goquery"
	"github.com/google/go-github/v74/github"
	"golang.org/x/oauth2"
)

type MetaRepo struct {
	StarredAt github.Timestamp `json:"starred_at,omitempty"`
	// ID        int64            `json:"id,omitempty"`
	// NodeID          string           `json:"node_id,omitempty"`
	OwnerName   string `json:"ownerName,omitempty"`
	Name        string `json:"name,omitempty"`
	FullName    string `json:"full_name,omitempty"`
	Description string `json:"description,omitempty"`
	// Homepage        string           `json:"homepage,omitempty"`
	// DefaultBranch string `json:"default_branch,omitempty"`
	// MasterBranch    string           `json:"master_branch,omitempty"`
	CreatedAt github.Timestamp `json:"created_at,omitempty"`
	// PushedAt        github.Timestamp `json:"pushed_at,omitempty"`
	UpdatedAt       github.Timestamp `json:"updated_at,omitempty"`
	Language        string           `json:"language,omitempty"`
	Fork            bool             `json:"fork,omitempty"`
	ForksCount      int              `json:"forks_count,omitempty"`
	StargazersCount int              `json:"stargazers_count,omitempty"`
	// WatchersCount   int              `json:"watchers_count,omitempty"`
	LicenseName string `json:"licenseName,omitempty"`
	HTMLURL     string `json:"htmlUrl,omitempty"`
}

var (
	ghClient *github.Client
)

func createGitHubClient(ctx context.Context, token string) {
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	tc := oauth2.NewClient(ctx, ts)
	ghClient = github.NewClient(tc)
}

func newMetaRepo(repo *github.StarredRepository) *MetaRepo {
	return &MetaRepo{
		StarredAt: repo.GetStarredAt(),
		// ID:        repo.Repository.GetID(),
		// NodeID:          repo.Repository.GetNodeID(),
		OwnerName:   repo.Repository.GetOwner().GetName(),
		Name:        repo.Repository.GetName(),
		FullName:    repo.Repository.GetFullName(),
		Description: repo.Repository.GetDescription(),
		// Homepage:        repo.Repository.GetHomepage(),
		// DefaultBranch: repo.Repository.GetDefaultBranch(),
		// MasterBranch:    repo.Repository.GetMasterBranch(),
		CreatedAt: repo.Repository.GetCreatedAt(),
		// PushedAt:        repo.Repository.GetPushedAt(),
		UpdatedAt:       repo.Repository.GetUpdatedAt(),
		Language:        repo.Repository.GetLanguage(),
		Fork:            repo.Repository.GetFork(),
		ForksCount:      repo.Repository.GetForksCount(),
		StargazersCount: repo.Repository.GetStargazersCount(),
		// WatchersCount:   repo.Repository.GetWatchersCount(),
		LicenseName: repo.Repository.GetLicense().GetName(),
		HTMLURL:     repo.Repository.GetHTMLURL(),
	}
}

func ghFetchStarred(ctx context.Context, username string, page int, limit int) ([]*MetaRepo, error) {
	perPage := 100
	if limit > 0 {
		perPage = limit
		page = 1
	}

	opts := &github.ActivityListStarredOptions{
		Sort:        "created",
		Direction:   "desc",
		ListOptions: github.ListOptions{PerPage: perPage, Page: page},
	}

	var results []*MetaRepo

	for {
		starred, resp, err := ghClient.Activity.ListStarred(ctx, username, opts)
		if err != nil {
			log.Printf("Error fetching starred repos: %v", err)
			break
		}

		for _, star := range starred {
			if star.Repository != nil {
				results = append(results, newMetaRepo(star))
			}
		}

		if resp.NextPage == 0 || page > 0 {
			break
		}

		opts.Page = resp.NextPage
	}

	return results, nil
}

func ghSetStarred(ctx context.Context, owner string, repo string) error {
	_, err := ghClient.Activity.Star(ctx, owner, repo)
	if err != nil {
		log.Printf("Error starring repo: %v", err)
		return err
	}

	return nil
}

func ghSetUnstarred(ctx context.Context, owner string, repo string) error {
	_, err := ghClient.Activity.Unstar(ctx, owner, repo)
	if err != nil {
		log.Printf("Error unstarring repo: %v", err)
		return err
	}

	return nil
}

func ghGetLists(ctx context.Context, username string) ([]string, error) {
	if len(username) == 0 {
		user, _, err := ghClient.Users.Get(ctx, username)
		if err != nil {
			log.Printf("Error fetching %s user: %v", username, err)
			return nil, err
		}

		username = user.GetLogin()
	}

	res, err := http.Get(fmt.Sprintf("https://github.com/%s?tab=stars&user_lists_direction=asc&user_lists_sort=name", username))
	if err != nil {
		log.Printf("Error fetching user (%s) stars page: %v", username, err)
		return nil, err
	}
	defer res.Body.Close()

	var results []string

	if res.StatusCode != http.StatusOK {
		log.Printf("Error fetching user (%s) stars page: (%d) %s",
			username, res.StatusCode, res.Status)
		return results, nil
	}

	doc, err := goquery.NewDocumentFromReader(res.Body)
	if err != nil {
		log.Printf("Error loading html page DOM: %v", err)
		return nil, err
	}

	doc.Find("#profile-lists-container h3").Each(func(i int, s *goquery.Selection) {
		results = append(results, s.Text())
	})

	return results, nil
}
