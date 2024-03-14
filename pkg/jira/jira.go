package jira

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"

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
	Client      JiraClient
	CurrentUser *jira.User
}

func NewConfig(host string, token string, username string) (*Config, error) {
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
		return &c, err
	}

	client := c.Client.(*jira.Client)
	c.CurrentUser, err = GetUser(client.User, username)
	if err != nil {
		return &c, err
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
