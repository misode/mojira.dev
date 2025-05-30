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
	Date       *time.Time
	Author     string
	AvatarUrl  string
	AdfComment string
}

func (a *Attachment) IsImage() bool {
	return strings.HasPrefix(a.MimeType, "image/")
}
