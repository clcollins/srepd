package jira

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"

	"github.com/andygrunwald/go-jira"
)

type JiraClientInterface interface {
	NewRawRequestWithContext(ctx context.Context, method, urlStr string, body io.Reader) (*http.Request, error)
	NewRawRequest(method, urlStr string, body io.Reader) (*http.Request, error)
	NewRequestWithContext(ctx context.Context, method, urlStr string, body interface{}) (*http.Request, error)
	NewRequest(method, urlStr string, body interface{}) (*http.Request, error)
	NewMultiPartRequestWithContext(ctx context.Context, method, urlStr string, buf *bytes.Buffer) (*http.Request, error)
	NewMultiPartRequest(method, urlStr string, buf *bytes.Buffer) (*http.Request, error)
	Do(req *http.Request, v interface{}) (*jira.Response, error)
	GetBaseURL() url.URL
}

type JiraClient interface {
	JiraClientInterface
}

type Config struct {
	Client        JiraClient
	CurrentUser   *jira.User
	DefaultFilter *jira.Filter
	DefaultBoard  *jira.Board
	CustomJql     string
}

func NewConfig(host string, token string, username string, boardID int, jql string) (*Config, error) {
	var c Config
	var err error

	var tc *http.Client
	if username == "" {
		log.Printf("jira.NewConfig(): using PAT")
		transport := jira.PATAuthTransport{
			Token: token,
		}
		tc = transport.Client()
	} else {
		log.Printf("jira.NewConfig(): using BasicAuth")
		transport := jira.BasicAuthTransport{
			Username: username,
			Password: token,
		}
		tc = transport.Client()
	}

	c.Client, err = newClient(tc, host)
	if err != nil {
		return &c, fmt.Errorf("jira.NewConfig(): error creating client: %v", err)
	}

	client := c.Client.(*jira.Client)

	c.CurrentUser, _, err = client.User.GetSelf()
	if err != nil {
		return &c, fmt.Errorf("jira.NewConfig(): error getting current user: %v", err)
	}

	c.DefaultBoard, err = GetBoard(client, boardID)
	if err != nil {
		return &c, fmt.Errorf("jira.NewConfig(): error getting board: %v", err)
	}

	f, err := GetBoardConfigurationFilter(client, boardID)
	if err != nil {
		return &c, fmt.Errorf("jira.NewConfig(): error getting board configuration filter: %v", err)
	}

	// *jira.BoardConfigurationFilter.ID is a string for some reason
	fint, err := strconv.Atoi(f.ID)
	if err != nil {
		return &c, fmt.Errorf("jira.NewConfig(): error converting filter ID to int: %v", err)
	}

	c.DefaultFilter, err = GetFilter(client, fint)
	if err != nil {
		return &c, fmt.Errorf("jira.NewConfig(): error getting filter: %v", err)
	}

	c.CustomJql = jql

	return &c, nil
}

func newClient(t *http.Client, h string) (JiraClient, error) {
	c, err := jira.NewClient(t, h)
	if err != nil {
		return nil, err
	}
	return c, nil
}

// Get Board returns the Jira board associated with the ID string
func GetBoard(client *jira.Client, boardID int) (*jira.Board, error) {
	var b *jira.Board

	if boardID == 0 {
		return b, fmt.Errorf("jira.GetBoardConfigurationFilter(): boardID is unset or invalid: %v", boardID)
	}

	b, resp, err := client.Board.GetBoard(boardID)
	if err != nil {
		log.Printf("jira.GetBoard(): resp: %+v", resp)
		return b, fmt.Errorf("jira.GetBoard(): failed to get board: %v", err)
	}

	return b, nil
}

// GetBoardConfigurationFilter returns the base filter associated with the given board
func GetBoardConfigurationFilter(client *jira.Client, boardID int) (jira.BoardConfigurationFilter, error) {
	var f jira.BoardConfigurationFilter

	if boardID == 0 {
		return f, fmt.Errorf("jira.GetBoardConfigurationFilter(): boardID is unset or invalid: %v", boardID)
	}

	b, resp, err := client.Board.GetBoardConfiguration(boardID)
	if err != nil {
		log.Printf("jira.GetBoardConfigurationFilter(): resp: %+v", resp)
		return f, fmt.Errorf("jira.GetBoardDefaultFilterID(): failed to get board configuration: %v, %v", boardID, err)
	}

	f = b.Filter

	// FYI: *jira.BoardConfigurationFilter.ID is a string for some reason, not an int
	return f, nil
}

func GetFilter(client *jira.Client, filterID int) (*jira.Filter, error) {
	var f *jira.Filter

	if filterID == 0 {
		return f, fmt.Errorf("jira.GetFilter(): filterID is unset or invalid: %v", filterID)
	}

	f, resp, err := client.Filter.Get(filterID)
	if err != nil {
		log.Printf("jira.GetFilter(): resp: %+v", resp)
		return nil, fmt.Errorf("jira.GetFilter(): failed to get filter: %v, %v", filterID, err)
	}

	// FYI: *jira.Filter.ID is a string for some reason, not an int
	return f, nil
}

func GetIssues(client *jira.Client, jql string) ([]jira.Issue, error) {
	var i []jira.Issue

	opts := jira.SearchOptions{
		MaxResults: 1000,
	}

	for {
		chunk, resp, err := client.Issue.Search(jql, &opts)
		if err != nil {
			return nil, fmt.Errorf("jira.GetIssues(): failed to get issues: %v", err)
		}

		i = append(i, chunk...)

		opts.StartAt += opts.MaxResults

		if opts.StartAt >= resp.Total {
			break
		}
	}

	return i, nil
}

func GetUser(userService *jira.UserService, username string) (*jira.User, error) {
	users, resp, err := userService.Find(username)
	if err != nil {
		log.Printf("jira.GetUser(): error: %+v", err)
		log.Printf("jira.GetUser(): response.Status: %+v", resp.Status)
		log.Printf("jira.GetUser(): response.Body: %+v", resp.Body)
		return nil, err
	}

	if len(users) != 1 {
		return nil, fmt.Errorf("jira.GetUser(): error finding user %v: expected 1 user but found %v", username, len(users))
	}

	return &users[0], nil
}

type UserService interface {
	Find(property string, tweaks ...func([]userSearchParam) []userSearchParam) ([]jira.User, *jira.Response, error)
}

type userSearchParam struct {
	name  string
	value string
}
