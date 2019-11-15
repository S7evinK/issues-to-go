package gh

import (
	"context"
	"fmt"
	"github.com/pkg/errors"
	github "github.com/shurcooL/githubv4"
	"golang.org/x/oauth2"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type (
	GH struct {
		client    *github.Client
		tz        *time.Location
		user      string
		repo      string
		output    string
		since     time.Time
		all       bool
		count     int
		variables map[string]interface{}
		states    []github.IssueState
	}

	IssueConnection struct {
		Edges    []IssueEdge `graphql:"edges"`
		PageInfo PageInfo    `graphql:"pageInfo"`
	}

	PageInfo struct {
		EndCursor   github.String
		HasNextPage bool
	}

	IssueEdge struct {
		Cursor string `graphql:"cursor"`
		Node   Issue  `graphql:"node"`
	}

	Issue struct {
		ID        string    `graphql:"id"`
		Number    int       `graphql:"number"`
		Body      string    `graphql:"body"`
		Title     string    `graphql:"title"`
		Author    Author    `graphql:"author"`
		CreatedAt time.Time `graphql:"createdAt"`
		Milestone Milestone `graphql:"milestone"`
		Comments  Comments  `graphql:"comments(first: $count, after: $commentsCursor )"`
		State     string    `graphql:"state"`
		Closed    bool      `graphql:"closed"`
		ClosedAt  time.Time `graphql:"closedAt"`
	}

	Author struct {
		Name string `graphql:"login"`
	}

	Milestone struct {
		Title string `graphql:"title"`
	}

	Comments struct {
		Nodes    []Comment
		PageInfo PageInfo `graphql:"pageInfo"`
	}

	Comment struct {
		Body   string
		Author struct {
			Login string
		}
		CreatedAt time.Time `graphql:"createdAt"`
	}

	Query struct {
		Repository struct {
			IssueConnection IssueConnection `graphql:"issues(first: $count, after: $issueCursor, filterBy: $filterBy )"`
		} `graphql:"repository(owner: $owner, name: $name)"`
	}

	QueryComments struct {
		Repository struct {
			Issue Issue `graphql:"issue(number: $issueNumber)"`
		} `graphql:"repository(owner: $owner, name: $name)"`
	}
)

type Error string

func (e Error) Error() string { return string(e) }

const ErrNoIssues = Error("no new or updated issues found")

// New creates a new github v4 client and prepares the folders and queries
func New(token, user, repo, output string, count int, all bool, since time.Time, tz *time.Location) (*GH, error) {
	src := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)

	httpClient := oauth2.NewClient(context.Background(), src)
	httpClient.Timeout = 30 * time.Second

	client := github.NewClient(httpClient)

	variables := map[string]interface{}{
		"owner":          github.String(user),
		"name":           github.String(repo),
		"issueCursor":    (*github.String)(nil),
		"commentsCursor": (*github.String)(nil),
		"count":          github.Int(count),
	}

	gh := &GH{
		client:    client,
		user:      user,
		repo:      repo,
		output:    output,
		tz:        tz,
		since:     since,
		all:       all,
		count:     count,
		variables: variables,
	}

	if err := gh.createDirs(); err != nil {
		return nil, errors.Wrap(err, "unable to create directories")
	}

	return gh, nil
}

// FetchIssues gets all requested issues from a given repository.
func (gh *GH) FetchIssues() error {
	var (
		count = 0
		since = gh.since
		tz    = gh.tz
		q     Query
	)
	gh.states = []github.IssueState{github.IssueStateOpen}

	if gh.all {
		gh.states = append(gh.states, github.IssueStateClosed)
	}

	gh.variables["filterBy"] = github.IssueFilters{Since: &github.DateTime{since.UTC()}, States: &gh.states}

	for {
		err := gh.client.Query(context.Background(), &q, gh.variables)
		if err != nil {
			return err
		}

		if len(q.Repository.IssueConnection.Edges) == 0 {
			return ErrNoIssues
		}

		for _, issue := range q.Repository.IssueConnection.Edges {
			comments, err := gh.extractComments(&issue, tz)
			if err != nil {
				return errors.Wrap(err, "unable to extract comments")
			}
			if issue.Node.Closed {
				footer := []byte(fmt.Sprintf("Closed on %v", issue.Node.ClosedAt.In(tz)))
				comments = append(comments, footer...)
			}
			outputFile := filepath.Join(gh.output, strings.ToLower(issue.Node.State), strconv.Itoa(issue.Node.Number)+".md")
			if err := ioutil.WriteFile(outputFile, comments, os.ModePerm); err != nil {
				return errors.Wrap(err, fmt.Sprintf("error writing issue %d", issue.Node.Number))
			}
			count++
		}

		// break endless loop if we're on the last page
		if !q.Repository.IssueConnection.PageInfo.HasNextPage {
			break
		}

		gh.variables["issueCursor"] = q.Repository.IssueConnection.PageInfo.EndCursor
	}

	log.Printf("Downloaded %d issue(s) including comments", count)

	return nil
}

func (gh *GH) extractComments(issue *IssueEdge, tz *time.Location) ([]byte, error) {
	var (
		result    []byte
		q         QueryComments
		comments  = issue.Node.Comments
		regex     = regexp.MustCompile(`(#(\d+))`)
		variables = map[string]interface{}{
			"issueNumber": github.Int(issue.Node.Number),
			"count":       github.Int(gh.count),
			"owner":       github.String(gh.user),
			"name":        github.String(gh.repo),
		}
	)

	header := []byte(
		fmt.Sprintf("%s\n---\n\nCreated by %s on %v:\n\n%s\n\n---\n",
			issue.Node.Title,
			issue.Node.Author.Name,
			issue.Node.CreatedAt.In(tz),
			regex.ReplaceAllString(issue.Node.Body, "[#$2]($2.md)"),
		),
	)

	result = append(result, header...)

	for {
		for _, com := range comments.Nodes {
			b := []byte(fmt.Sprintf("\n%s commented on %v:\n\n%s\n\n---\n",
				com.Author.Login,
				com.CreatedAt.In(tz),
				regex.ReplaceAllString(com.Body, "[#$2]($2.md)"),
				//com.Body,
			),
			)
			result = append(result, b...)
		}

		// break endless loop if we're on the last page
		if !comments.PageInfo.HasNextPage {
			break
		}

		variables["commentsCursor"] = comments.PageInfo.EndCursor

		err := gh.client.Query(context.Background(), &q, variables)
		if err != nil {
			return nil, err
		}

		comments = q.Repository.Issue.Comments

		log.Println("Getting next page of comments")
	}

	return result, nil
}

func (gh *GH) createDirs() error {
	if err := os.MkdirAll(gh.output, os.ModePerm); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(gh.output, "open"), os.ModePerm); err != nil {
		return err
	}
	if gh.all {
		if err := os.MkdirAll(filepath.Join(gh.output, "closed"), os.ModePerm); err != nil {
			return err
		}
	}
	return nil
}
