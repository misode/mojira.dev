package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mojira/model"
	"net/http"
	"strings"
	"time"
)

type PublicIssue struct {
	Key                string
	Summary            string
	Description        string
	Labels             []string
	CreatedDate        *time.Time
	UpdatedDate        *time.Time
	ResolvedDate       *time.Time
	Status             string
	ConfirmationStatus string
	Resolution         string
	AffectedVersions   []string
	FixVersions        []string
	Category           []string
	MojangPriority     string
	Area               string
	Platform           string
	OSVersion          string
	RealmsPlatform     string
	ADO                string
	Votes              int
	Links              []model.IssueLink
	Attachments        []model.Attachment
}

type PublicClient struct {
	client *http.Client
}

func NewPublicClient() *PublicClient {
	return &PublicClient{
		client: &http.Client{Timeout: 4 * time.Second},
	}
}

type publicJQLRequest struct {
	Advanced   bool   `json:"advanced"`
	Project    string `json:"project"`
	Search     string `json:"search"`
	MaxResults int    `json:"maxResults"`
}

func (c *PublicClient) GetIssue(ctx context.Context, key string) (*PublicIssue, error) {
	body, _ := json.Marshal(publicJQLRequest{
		Advanced:   true,
		Project:    strings.Split(key, "-")[0],
		Search:     "key = " + key,
		MaxResults: 1,
	})
	url := "https://bugs.mojang.com/api/jql-search-post"
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var parsed struct {
		Issues []struct {
			Key    string
			Fields struct {
				Summary     string
				Description any
				Status      struct {
					Name string
				}
				ConfirmationStatus struct {
					Value string
				} `json:"customfield_10054"`
				Area struct {
					Value string
				} `json:"customfield_10051"`
				Resolution struct {
					Name string
				}
				ResolutionDate string
				Labels         []string
				Category       []struct {
					Value string
				} `json:"customfield_10055"`
				MojangPriority struct {
					Value string
				} `json:"customfield_10049"`
				ADO      string `json:"customfield_10050"`
				Platform struct {
					Value string
				} `json:"customfield_10063"`
				OSVersion      string `json:"customfield_10061"`
				RealmsPlatform struct {
					Value string
				} `json:"customfield_10056"`
				Votes    int `json:"customfield_10070"`
				Created  string
				Updated  string
				Versions []struct {
					Name string
				}
				FixVersions []struct {
					Name string
				}
				Attachment []struct {
					Id       string
					Filename string
					Author   struct {
						DisplayName string
						AvatarUrls  struct {
							Size48 string `json:"48x48"`
						}
					}
					Created  string
					Size     int64
					MimeType string
				}
				IssueLinks []struct {
					Type struct {
						Inward  string
						Outward string
					}
					InwardIssue struct {
						Key    string
						Fields struct {
							Summary string
							Status  struct {
								Name string
							}
						}
					}
					OutwardIssue struct {
						Key    string
						Fields struct {
							Summary string
							Status  struct {
								Name string
							}
						}
					}
				}
			}
		}
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, err
	}
	if len(parsed.Issues) == 0 {
		return nil, errors.New("issue not found on public API")
	}

	f := parsed.Issues[0].Fields
	var versions []string
	for _, v := range f.Versions {
		versions = append(versions, v.Name)
	}
	var fixVersions []string
	for _, v := range f.FixVersions {
		fixVersions = append(fixVersions, v.Name)
	}
	desc := ""
	switch d := f.Description.(type) {
	case string:
		desc = d
	case map[string]any:
		b, _ := json.Marshal(d)
		desc = string(b)
	}
	var category []string
	for _, c := range f.Category {
		category = append(category, c.Value)
	}
	var links []model.IssueLink
	for _, l := range f.IssueLinks {
		link := model.IssueLink{}
		if l.OutwardIssue.Key != "" {
			link.Type = l.Type.Outward
			link.OtherKey = l.OutwardIssue.Key
			link.OtherSummary = l.OutwardIssue.Fields.Summary
			link.OtherStatus = l.OutwardIssue.Fields.Status.Name
			links = append(links, link)
		} else if l.InwardIssue.Key != "" {
			link.Type = l.Type.Inward
			link.OtherKey = l.InwardIssue.Key
			link.OtherSummary = l.InwardIssue.Fields.Summary
			link.OtherStatus = l.InwardIssue.Fields.Status.Name
			links = append(links, link)
		}
	}
	var attachments []model.Attachment
	for _, a := range f.Attachment {
		created, err := ParseTime(a.Created)
		if err != nil {
			return nil, err
		}
		attachments = append(attachments, model.Attachment{
			Id:           a.Id,
			Filename:     a.Filename,
			AuthorName:   a.Author.DisplayName,
			AuthorAvatar: a.Author.AvatarUrls.Size48,
			CreatedDate:  created,
			Size:         a.Size,
			MimeType:     a.MimeType,
		})
	}
	var createdDate, updatedDate, resolvedDate *time.Time
	createdDate, err = ParseTime(f.Created)
	if err != nil {
		return nil, err
	}
	updatedDate, err = ParseTime(f.Updated)
	if err != nil {
		return nil, err
	}
	resolvedDate, err = ParseTime(f.ResolutionDate)
	if err != nil {
		return nil, err
	}
	return &PublicIssue{
		Key:                key,
		Summary:            f.Summary,
		Description:        desc,
		Labels:             f.Labels,
		CreatedDate:        createdDate,
		UpdatedDate:        updatedDate,
		ResolvedDate:       resolvedDate,
		Status:             f.Status.Name,
		ConfirmationStatus: f.ConfirmationStatus.Value,
		Resolution:         f.Resolution.Name,
		AffectedVersions:   versions,
		FixVersions:        fixVersions,
		Category:           category,
		MojangPriority:     f.MojangPriority.Value,
		Area:               f.Area.Value,
		Platform:           strings.TrimSpace(f.Platform.Value),
		OSVersion:          f.OSVersion,
		RealmsPlatform:     f.RealmsPlatform.Value,
		ADO:                f.ADO,
		Votes:              f.Votes,
		Links:              links,
		Attachments:        attachments,
	}, nil
}
