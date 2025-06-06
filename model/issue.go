package model

import (
	"encoding/json"
	"fmt"
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
	Components         []string
	Platform           string
	OSVersion          string
	RealmsPlatform     string
	ADO                string
	Votes              int
	Links              []IssueLink
	Attachments        []Attachment
	Comments           []Comment
	SyncedDate         *time.Time
	Partial            bool
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

var PortalIds = map[string]int{
	"MC":     2,
	"MCPE":   6,
	"MCL":    7,
	"REALMS": 9,
	"WEB":    10,
	"BDS":    4,
}

func (i *Issue) PortalId() int {
	return PortalIds[i.Project()]
}

func (i *Issue) IsProject(projects ...string) bool {
	return slices.Contains(projects, i.Project())
}

func (i *Issue) IsResolved() bool {
	return i.Status == "Resolved" || i.Status == "Closed"
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
	if i.AffectedVersions == nil {
		return ""
	}
	n := len(i.AffectedVersions)
	if n <= 10 {
		return strings.Join(i.AffectedVersions, ", ")
	}
	short := append(i.AffectedVersions[:5], "...")
	short = append(short, i.AffectedVersions[n-5:]...)
	return strings.Join(short, ", ")
}

func (i *Issue) IsUpToDate() bool {
	if i.SyncedDate == nil {
		return true
	}
	offset := time.Duration(-5) * time.Minute
	return i.SyncedDate.After(time.Now().Add(offset))
}

func (i *Issue) FirstImage() *Attachment {
	for _, a := range i.Attachments {
		if a.IsImage() {
			return &a
		}
	}
	return nil
}

func (a *Attachment) IsImage() bool {
	return strings.HasPrefix(a.MimeType, "image/")
}

func (a *Attachment) GetUrl() string {
	return fmt.Sprintf("https://bugs.mojang.com/api/issue-attachment-get?attachmentId=%s", a.Id)
}
