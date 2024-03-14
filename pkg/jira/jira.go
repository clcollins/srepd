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
}

func NewConfig(host string, token string, username string, filter string) (*Config, error) {
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
	log.Printf("jira.NewConfig(): %v", c.Client)
	if err != nil {
		return &c, fmt.Errorf("jira.NewConfig(): error creating client: %v", err)
	}

	client := c.Client.(*jira.Client)

	c.CurrentUser, _, err = client.User.GetSelf()
	if err != nil {
		return &c, fmt.Errorf("jira.NewConfig(): error getting current user: %v", err)
	}

	c.DefaultFilter, err = GetFilter(client, filter)
	if err != nil {
		return &c, fmt.Errorf("jira.NewConfig(): error getting default filter: %v", err)
	}

	return &c, nil
}

func newClient(t *http.Client, h string) (JiraClient, error) {
	c, err := jira.NewClient(t, h)
	if err != nil {
		return nil, err
	}
	return c, nil
}

func GetFilter(client *jira.Client, filterID string) (*jira.Filter, error) {
	// Convert filterID to int because the jira.Filter.Get() method expects an int for some reason
	// even though *jira.Filter.ID is a string
	i, err := strconv.Atoi(filterID)
	if err != nil {
		return nil, fmt.Errorf("jira.GetFilter(): failed to convert filterID to int: %v", err)
	}

	f, _, err := client.Filter.Get(i)
	if err != nil {
		return nil, fmt.Errorf("jira.GetFilter(): failed to get filter: %v", err)
	}
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
