package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"mojira/model"
)

type ServiceDeskClient struct {
	client *http.Client
	cookie *http.Cookie
}

var portalIds = map[string]int{
	"MC":     2,
	"MCPE":   6,
	"MCL":    7,
	"REALMS": 9,
	"WEB":    10,
	"BDS":    4,
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

func (s *ServiceDeskClient) GetIssue(ctx context.Context, key string) (*model.Issue, error) {
	portalId := portalIds[strings.Split(key, "-")[0]]
	body, err := json.Marshal(map[string]interface{}{
		"models": []string{"reqDetails"},
		"options": map[string]interface{}{
			"reqDetails": map[string]interface{}{
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
				Summary string
				Status  string
				Date    string
				Fields  []struct {
					Id    string
					Value json.RawMessage
				}
				ActivityStream []struct {
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
		var date *time.Time
		if c.Date != "" {
			t, err := ParseTime(c.Date)
			if err != nil {
				return nil, err
			}
			date = t
		}
		comments = append(comments, model.Comment{
			Date:       date,
			Author:     c.Author,
			AvatarUrl:  c.AvatarUrl,
			AdfComment: c.AdfComment,
		})
	}

	description := ""
	affectedVersions := ""
	environment := ""
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
	return &model.Issue{
		Key:                key,
		Summary:            apiIssue.Summary,
		ReporterName:       apiIssue.Reporter.DisplayName,
		ReporterAvatar:     apiIssue.Reporter.AvatarUrl,
		Description:        description,
		Environment:        environment,
		Labels:             "",
		CreatedDate:        createdDate,
		UpdatedDate:        nil,
		ResolvedDate:       nil,
		Status:             apiIssue.Status,
		ConfirmationStatus: "",
		Resolution:         "",
		AffectedVersions:   affectedVersions,
		FixVersions:        "",
		MojangPriority:     "",
		Area:               "",
		Comments:           comments,
	}, nil
}

func (s *ServiceDeskClient) GetUpdatedIssues(ctx context.Context) ([]string, error) {
	body, err := json.Marshal(map[string]interface{}{
		"models": []string{"allReqFilter"},
		"options": map[string]interface{}{
			"allReqFilter": map[string]interface{}{
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
