package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"mojira/model"
)

type ServiceDeskIssue struct {
	Key              string
	Summary          string
	ReporterName     string
	ReporterAvatar   string
	AssigneeName     string
	AssigneeAvatar   string
	Description      string
	Environment      string
	CreatedDate      *time.Time
	Status           string
	AffectedVersions []string
	Components       []string
	RealmsPlatform   string
	Comments         []model.Comment
}

type ServiceDeskClient struct {
	client *http.Client
	cookie *http.Cookie
}

func NewServiceDeskClient(ctx context.Context) (*ServiceDeskClient, error) {
	body, err := json.Marshal(map[string]string{
		"email":    os.Getenv("JIRA_EMAIL"),
		"password": os.Getenv("JIRA_PASSWORD"),
	})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://report.bugs.mojang.com/jsd-login/v1/authentication/authenticate", bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	cookies := resp.Cookies()
	if len(cookies) == 0 {
		return nil, errors.New("no session cookie returned")
	}

	return &ServiceDeskClient{
		client: client,
		cookie: cookies[0],
	}, nil
}

func (s *ServiceDeskClient) GetIssue(ctx context.Context, key string) (*ServiceDeskIssue, error) {
	portalId := model.PortalIds[strings.Split(key, "-")[0]]
	body, err := json.Marshal(map[string]any{
		"models": []string{"reqDetails"},
		"options": map[string]any{
			"reqDetails": map[string]any{
				"key":      key,
				"portalId": portalId,
			},
			"portalId": portalId,
		},
	})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://report.bugs.mojang.com/rest/servicedesk/1/customer/models", bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(s.cookie)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var parsed struct {
		ReqDetails struct {
			Issue struct {
				Key      string
				Reporter struct {
					DisplayName string
					AvatarUrl   string
				}
				Assignee struct {
					DisplayName string
					AvatarUrl   string
				}
				Summary string
				Status  string
				Date    string
				Fields  []struct {
					Id    string
					Value json.RawMessage
				}
				ActivityStream []struct {
					Type       string
					CommentId  int
					Date       string
					Author     string
					AvatarUrl  string
					AdfComment string
				}
			}
		}
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, err
	}
	if parsed.ReqDetails.Issue.Key == "" {
		return nil, errors.New("issue not found on servicedesk API")
	}

	apiIssue := parsed.ReqDetails.Issue
	comments := make([]model.Comment, 0, len(apiIssue.ActivityStream))
	for _, c := range apiIssue.ActivityStream {
		if c.Type != "worker-comment" && c.Type != "requester-comment" {
			continue
		}
		var date *time.Time
		if c.Date != "" {
			t, err := ParseTime(c.Date)
			if err != nil {
				return nil, err
			}
			date = t
		}
		comments = append(comments, model.Comment{
			Id:           strconv.Itoa(c.CommentId),
			Date:         date,
			AuthorName:   SafeName(c.Author),
			AuthorAvatar: c.AvatarUrl,
			AdfComment:   c.AdfComment,
		})
	}

	description := ""
	affectedVersions := ""
	environment := ""
	components := ""
	realmsPlatform := ""
	for _, f := range apiIssue.Fields {
		switch f.Id {
		case "description":
			var v struct {
				Adf string
			}
			_ = json.Unmarshal(f.Value, &v)
			description = v.Adf
		case "versions":
			var v struct {
				Text string
			}
			_ = json.Unmarshal(f.Value, &v)
			affectedVersions = v.Text
		case "environment":
			var v struct {
				Adf string
			}
			_ = json.Unmarshal(f.Value, &v)
			environment = v.Adf
		case "components":
			var v struct {
				Text string
			}
			_ = json.Unmarshal(f.Value, &v)
			components = v.Text
		case "customfield_10056":
			var v struct {
				Text string
			}
			_ = json.Unmarshal(f.Value, &v)
			realmsPlatform = v.Text
		}
	}

	var createdDate *time.Time
	if apiIssue.Date != "" {
		t, err := ParseTime(apiIssue.Date)
		if err != nil {
			return nil, err
		}
		createdDate = t
	}
	return &ServiceDeskIssue{
		Key:              key,
		Summary:          apiIssue.Summary,
		ReporterName:     SafeName(apiIssue.Reporter.DisplayName),
		ReporterAvatar:   apiIssue.Reporter.AvatarUrl,
		AssigneeName:     SafeName(apiIssue.Assignee.DisplayName),
		AssigneeAvatar:   apiIssue.Assignee.AvatarUrl,
		Description:      description,
		Environment:      environment,
		CreatedDate:      createdDate,
		Status:           apiIssue.Status,
		AffectedVersions: strings.Split(affectedVersions, ", "),
		Components:       strings.Split(components, ", "),
		RealmsPlatform:   realmsPlatform,
		Comments:         comments,
	}, nil
}

func (s *ServiceDeskClient) GetUpdatedIssues(ctx context.Context) ([]string, error) {
	body, err := json.Marshal(map[string]any{
		"models": []string{"allReqFilter"},
		"options": map[string]any{
			"allReqFilter": map[string]any{
				"selectedPage": 1,
				"reporter":     "all",
			},
		},
	})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://report.bugs.mojang.com/rest/servicedesk/1/customer/models", bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(s.cookie)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if strings.HasPrefix(string(raw), "<!DOCTYPE html>") {
		return nil, errors.New("received HTML response, likely rate limited")
	}

	var response struct {
		AllReqFilter struct {
			RequestList []struct {
				Key string
			}
		}
	}
	if err := json.Unmarshal(raw, &response); err != nil {
		return nil, err
	}

	var keys []string
	for _, i := range response.AllReqFilter.RequestList {
		keys = append(keys, i.Key)
	}
	return keys, nil
}
