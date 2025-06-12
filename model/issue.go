package model

import (
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"
)

type Issue struct {
	Key                string
	Summary            string
	CreatorName        string // Legacy
	CreatorAvatar      string // Legacy
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
	LegacyVotes        int // Legacy
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
	LegacyId     string // Legacy
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
	return i.Environment != "" && !IsEmptyADF(i.Environment)
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

func (i *Issue) TotalVotes() int {
	return i.LegacyVotes + i.Votes
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

type IssueLinkGroup struct {
	Type        string
	Count       int
	Links       []IssueLink
	HiddenLinks []IssueLink
}

func (i *Issue) GroupedLinks() []IssueLinkGroup {
	groupsMap := make(map[string][]IssueLink)
	for _, link := range i.Links {
		groupsMap[link.Type] = append(groupsMap[link.Type], link)
	}
	var groupedLinks []IssueLinkGroup
	for typ, links := range groupsMap {
		count := len(links)
		var hiddenLinks []IssueLink
		if count > 5 {
			hiddenLinks = links[5:]
			links = links[:5]
		}
		groupedLinks = append(groupedLinks, IssueLinkGroup{Type: typ, Count: count, Links: links, HiddenLinks: hiddenLinks})
	}
	sort.Slice(groupedLinks, func(a, b int) bool {
		return groupedLinks[a].Type < groupedLinks[b].Type
	})
	return groupedLinks
}

type VisibleComments struct {
	Count  int
	Top    []Comment
	Middle []Comment
	Bottom []Comment
}

func (i *Issue) VisibleComments() VisibleComments {
	var comments []Comment

	// Filter out first comment with only media
	if len(i.Comments) == 0 {
		comments = i.Comments
	} else {
		first := i.Comments[0]
		if first.Date.Sub(*i.CreatedDate) <= 2*time.Second && IsOnlyMediaADF(first.AdfComment) {
			comments = i.Comments[1:]
		} else {
			comments = i.Comments
		}
	}

	n := len(comments)
	if n <= 10 {
		return VisibleComments{Count: n, Top: comments}
	}
	return VisibleComments{
		Count:  n,
		Top:    comments[:5],
		Middle: comments[5 : n-5],
		Bottom: comments[n-5:],
	}
}

func (l *IssueLink) IsResolved() bool {
	return l.OtherStatus == "Resolved" || l.OtherStatus == "Closed"
}

func (a *Attachment) IsImage() bool {
	return strings.HasPrefix(a.MimeType, "image/")
}

func (a *Attachment) GetUrl() string {
	return fmt.Sprintf("https://bugs.mojang.com/api/issue-attachment-get?attachmentId=%s", a.Id)
}

func (c *Comment) Anchor() string {
	if c.LegacyId != "" {
		return fmt.Sprintf("comment-%s", c.LegacyId)
	}
	return fmt.Sprintf("comment-id-%s", c.Id)
}
