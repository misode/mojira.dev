package model

import (
	"encoding/json"
	"slices"
	"strings"
	"time"
)

type Issue struct {
	Key                string
	Summary            string
	ReporterName       string
	ReporterAvatar     string
	AssigneeName       string
	AssigneeAvatar     string
	Description        string
	Environment        string
	Labels             string
	CreatedDate        *time.Time
	UpdatedDate        *time.Time
	ResolvedDate       *time.Time
	Status             string
	ConfirmationStatus string
	Resolution         string
	AffectedVersions   string
	FixVersions        string
	Category           string
	MojangPriority     string
	Area               string
	Components         string
	Platform           string
	OSVersion          string
	RealmsPlatform     string
	ADO                string
	Votes              int
	Links              []IssueLink
	Attachments        []Attachment
	Comments           []Comment
}

type IssueLink struct {
	Type         string
	OtherKey     string
	OtherSummary string
	OtherStatus  string
}

type Attachment struct {
	Id           string
	Filename     string
	AuthorName   string
	AuthorAvatar string
	CreatedDate  *time.Time
	Size         int64
	MimeType     string
}

type Comment struct {
	Id           string
	Date         *time.Time
	AuthorName   string
	AuthorAvatar string
	AdfComment   string
}

func (i *Issue) Project() string {
	return strings.Split(i.Key, "-")[0]
}

func (i *Issue) IsProject(projects ...string) bool {
	return slices.Contains(projects, i.Project())
}

func (i *Issue) IsResolved() bool {
	return i.Status == "Resolved" || i.Status == "Done" || i.Status == "Closed"
}

func (i *Issue) HasEnvironment() bool {
	if i.Environment == "" {
		return false
	}
	var node map[string]any
	err := json.Unmarshal([]byte(i.Environment), &node)
	if err != nil {
		return false
	}
	content, ok := node["content"].([]any)
	if !ok {
		return false
	}
	return len(content) > 0
}

func (i *Issue) ShortAffectedVersions() string {
	parts := strings.Split(i.AffectedVersions, ",")
	for idx, p := range parts {
		parts[idx] = strings.TrimSpace(p)
	}
	n := len(parts)
	if n <= 10 {
		return strings.Join(parts, ", ")
	}
	short := append(parts[:5], "...")
	short = append(short, parts[n-5:]...)
	return strings.Join(short, ", ")
}

func (a *Attachment) IsImage() bool {
	return strings.HasPrefix(a.MimeType, "image/")
}
