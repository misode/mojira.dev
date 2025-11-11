package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mojira/model"
	"net/http"
	"time"
)

type LegacyIssue struct {
	CreatorKey     string
	CreatorName    string
	CreatorAvatar  string
	ReporterKey    string
	ReporterName   string
	ReporterAvatar string
	ResolvedDate   *time.Time
	Votes          int
	Comments       []model.Comment
}

type LegacyClient struct {
	client *http.Client
}

func NewLegacyClient() *LegacyClient {
	return &LegacyClient{
		client: &http.Client{},
	}
}

func (l *LegacyClient) GetIssue(ctx context.Context, key string) (*LegacyIssue, error) {
	NewApiCall("legacy")

	url := fmt.Sprintf("https://bugs-legacy.mojang.com/rest/api/2/issue/%s", key)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, NewApiError("legacy", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := l.client.Do(req)
	if err != nil {
		return nil, NewApiError("legacy", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, NewApiError("legacy", err)
	}

	var parsed struct {
		Key    string
		Fields struct {
			Creator struct {
				Key         string
				DisplayName string
				AvatarUrls  struct {
					Size48 string `json:"48x48"`
				}
			}
			Reporter struct {
				Key         string
				DisplayName string
				AvatarUrls  struct {
					Size48 string `json:"48x48"`
				}
			}
			Resolutiondate string
			Votes          struct {
				Votes int
			}
			Comment struct {
				Comments []struct {
					Id     string
					Author struct {
						DisplayName string
						AvatarUrls  struct {
							Size48 string `json:"48x48"`
						}
					}
					Created string
				}
			}
		}
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, NewApiError("legacy", err)
	}
	if parsed.Key == "" {
		return nil, model.ErrIssueNotFound
	}

	var resolvedDate *time.Time
	if parsed.Fields.Resolutiondate != "" {
		t, err := ParseTime(parsed.Fields.Resolutiondate)
		if err != nil {
			return nil, NewApiError("legacy", err)
		}
		resolvedDate = t
	}
	comments := make([]model.Comment, 0, len(parsed.Fields.Comment.Comments))
	for _, c := range parsed.Fields.Comment.Comments {
		var date *time.Time
		if c.Created != "" {
			t, err := ParseTime(c.Created)
			if err != nil {
				return nil, NewApiError("legacy", err)
			}
			date = t
		}
		comments = append(comments, model.Comment{
			LegacyId:     c.Id,
			Date:         date,
			AuthorName:   SafeName(c.Author.DisplayName),
			AuthorAvatar: c.Author.AvatarUrls.Size48,
		})
	}

	return &LegacyIssue{
		CreatorKey:     parsed.Fields.Creator.Key,
		CreatorName:    parsed.Fields.Creator.DisplayName,
		CreatorAvatar:  parsed.Fields.Creator.AvatarUrls.Size48,
		ReporterKey:    parsed.Fields.Reporter.Key,
		ReporterName:   parsed.Fields.Reporter.DisplayName,
		ReporterAvatar: parsed.Fields.Reporter.AvatarUrls.Size48,
		ResolvedDate:   resolvedDate,
		Votes:          parsed.Fields.Votes.Votes,
		Comments:       comments,
	}, nil
}
