package gh

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	github "github.com/shurcooL/githubv4"
	"golang.org/x/oauth2"
)

type (
	GH struct {
		client    *github.Client
		opts      Options
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
		Comments  Comments  `graphql:"comments(first: $count, after: $commentsCursor)"`
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
			IssueConnection IssueConnection `graphql:"issues(first: $count, after: $issueCursor, filterBy: $filterBy)"`
		} `graphql:"repository(owner: $owner, name: $name)"`
	}

	QueryComments struct {
		Repository struct {
			Issue Issue `graphql:"issue(number: $issueNumber)"`
		} `graphql:"repository(owner: $owner, name: $name)"`
	}

	Option func(*Options) error
	// Options defines all available options for the application
	Options struct {
		Token      string
		User       string
		Repo       string
		OutputPath string
		Count      int
		AllIssues  bool
		Since      time.Time
		Milestones bool
		TZ         *time.Location
	}
)

type Error string

func (e Error) Error() string { return string(e) }

const (
	ErrNoIssues     = Error("no new or updated issues found")
	ErrNoRepository = Error("could not determine repository. Make sure it is in the format USER/REPOSITORY")
)

// Repo extracts the user and repo from a full repo name (eg. S7evinK/issues-to-go)
func Repo(r string) Option {
	return func(o *Options) error {
		s := strings.Split(r, "/")
		if len(s) != 2 {
			return ErrNoRepository
		}
		o.User = s[0]
		o.Repo = s[1]
		return nil
	}
}

// Token sets the Github access token and returns an option
func Token(t string) Option {
	return func(o *Options) error {
		o.Token = t
		return nil
	}
}

// Output sets the output folder and returns an option
func Output(t string) Option {
	return func(o *Options) error {
		o.OutputPath = t
		return nil
	}
}

// All sets the issues to download and returns an option
func All(a bool) Option {
	return func(o *Options) error {
		o.AllIssues = a
		return nil
	}
}

// Count sets the issue count to fetch at once and returns an option
func Count(i int) Option {
	return func(o *Options) error {
		if i <= 0 {
			return fmt.Errorf("invalid count value: expected count > 0")
		}
		o.Count = i
		return nil
	}
}

// UTC sets the timezone to use for dates and returns an option
func UTC(b bool) Option {
	return func(o *Options) error {
		var tz = time.UTC
		if !b {
			tz = time.Local
		}
		o.TZ = tz
		return nil
	}
}

// Since sets the time to use for filtering issues and returns an option
func Since(s string) Option {
	return func(o *Options) error {
		since, err := time.Parse(time.RFC3339, s)
		if err != nil {
			since = time.Unix(0, 0)
			log.Println("Unable to parse timestamp, using default value of", since)
		}
		o.Since = since
		return nil
	}
}

// Milestones sets the option to download milestones and returns an option
func Milestones(b bool) Option {
	return func(o *Options) error {
		o.Milestones = b
		return nil
	}
}

// New creates a new github v4 client and prepares the folders and queries
func New(opts ...Option) (*GH, error) {
	o := Options{}
	for _, opt := range opts {
		if err := opt(&o); err != nil {
			return nil, err
		}
	}

	src := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: o.Token},
	)

	httpClient := oauth2.NewClient(context.Background(), src)
	httpClient.Timeout = 30 * time.Second

	client := github.NewClient(httpClient)

	variables := map[string]interface{}{
		"owner":          github.String(o.User),
		"name":           github.String(o.Repo),
		"issueCursor":    (*github.String)(nil),
		"commentsCursor": (*github.String)(nil),
		"count":          github.Int(o.Count),
	}

	gh := &GH{
		client:    client,
		opts:      o,
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
		since = gh.opts.Since
		tz    = gh.opts.TZ
		q     Query
	)
	gh.states = []github.IssueState{github.IssueStateOpen}

	if gh.opts.AllIssues {
		gh.states = append(gh.states, github.IssueStateClosed)
	}

	gh.variables["filterBy"] = github.IssueFilters{Since: &github.DateTime{since.UTC()}, States: &gh.states}

	regexMilestones := regexp.MustCompile(`\/`)

	existing, err := readExistingIssues(gh.opts.OutputPath)
	if err != nil && err != os.ErrNotExist {
		return errors.Wrap(err, "unable to read existing issues")
	}

	var downloadedIssues []string
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
			// delete existing issues, since we'll write new ones
			if delPaths, ok := existing[strconv.Itoa(issue.Node.Number)+".md"]; ok {
				for _, path := range delPaths {
					if err := os.Remove(path); err != nil {
						return errors.Wrap(err, "unable to delete existing issue")
					}
				}
			}
			outputFile := filepath.Join(gh.opts.OutputPath, strings.ToLower(issue.Node.State), strconv.Itoa(issue.Node.Number)+".md")
			if err := ioutil.WriteFile(outputFile, comments, os.ModePerm); err != nil {
				return errors.Wrap(err, fmt.Sprintf("error writing issue %d", issue.Node.Number))
			}

			if err := gh.writeMilestone(&issue, regexMilestones, outputFile); err != nil {
				return errors.Wrap(err, fmt.Sprintf("error creating symlink for issue %d", issue.Node.Number))
			}

			downloadedIssues = append(downloadedIssues, outputFile)
			count++
		}

		// break endless loop if we're on the last page
		if !q.Repository.IssueConnection.PageInfo.HasNextPage {
			break
		}

		gh.variables["issueCursor"] = q.Repository.IssueConnection.PageInfo.EndCursor
	}

	log.Printf("Downloaded %d issue(s) including comments:", count)

	for _, fp := range downloadedIssues {
		fmt.Println(fp)
	}

	return nil
}

func (gh *GH) writeMilestone(issue *IssueEdge, regexMilestones *regexp.Regexp, outputFile string) error {
	if gh.opts.Milestones && issue.Node.Milestone.Title != "" {
		ms := regexMilestones.ReplaceAllString(issue.Node.Milestone.Title, "_")
		if err := gh.createMilestoneDir(ms); err != nil {
			return err
		}
		if err := gh.createSymlink(outputFile, ms, issue); err != nil {
			return err
		}
	}
	return nil
}

func (gh *GH) createSymlink(outputFile string, ms string, issue *IssueEdge) error {
	oldPath := filepath.Join(outputFile)
	if !filepath.IsAbs(oldPath) {
		oldPath = filepath.Join("..", "..", "..", "..", outputFile)
	}
	newPath := filepath.Join(gh.opts.OutputPath, "milestones", ms, strings.ToLower(issue.Node.State), strconv.Itoa(issue.Node.Number)+".md")
	if err := os.Symlink(oldPath, newPath); err != nil && !os.IsExist(err) {
		return err
	}
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
			"count":       github.Int(gh.opts.Count),
			"owner":       github.String(gh.opts.User),
			"name":        github.String(gh.opts.Repo),
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
	if err := os.MkdirAll(filepath.Join(gh.opts.OutputPath, "open"), os.ModePerm); err != nil {
		return err
	}
	if gh.opts.AllIssues {
		if err := os.MkdirAll(filepath.Join(gh.opts.OutputPath, "closed"), os.ModePerm); err != nil {
			return err
		}
	}
	return nil
}

func (gh *GH) createMilestoneDir(milestone string) error {
	if gh.opts.Milestones {
		if err := os.MkdirAll(filepath.Join(gh.opts.OutputPath, "milestones", milestone, "open"), os.ModePerm); err != nil {
			return err
		}
		if gh.opts.AllIssues {
			if err := os.MkdirAll(filepath.Join(gh.opts.OutputPath, "milestones", milestone, "closed"), os.ModePerm); err != nil {
				return err
			}
		}
	}
	return nil
}

func readExistingIssues(path string) (map[string][]string, error) {
	existing := make(map[string][]string)
	err := filepath.Walk(path, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		existing[info.Name()] = append(existing[info.Name()], path)
		return nil
	})
	return existing, err
}
