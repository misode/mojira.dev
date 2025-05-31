package model

import (
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

func (i *Issue) IsResolved() bool {
	return i.Status == "Resolved" || i.Status == "Done" || i.Status == "Closed"
}

func (a *Attachment) IsImage() bool {
	return strings.HasPrefix(a.MimeType, "image/")
}
