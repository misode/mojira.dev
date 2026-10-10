package main

import (
	"context"
	"errors"
	"log"
	"mojira/api"
	"mojira/model"
	"os"
	"strings"
	"time"
)

type IssueService struct {
	db           *DBClient
	public       *api.PublicClient
	serviceDesk  *api.ServiceDeskClient
	redactedKeys map[string]struct{}
}

func NewIssueService() *IssueService {
	dbClient, err := NewDBClient()
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	public := api.NewPublicClient()
	serviceDesk := api.NewServiceDeskClient()
	err = serviceDesk.Authenticate()
	if err != nil {
		log.Printf("Failed to authenticate to service desk: %v", err)
	}

	redactedKeys := make(map[string]struct{})
	data, err := os.ReadFile("redacted.txt")
	if err == nil {
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line != "" {
				redactedKeys[line] = struct{}{}
			}
		}
	}
	log.Printf("Using %v redacted keys", len(redactedKeys))

	return &IssueService{db: dbClient, public: public, serviceDesk: serviceDesk, redactedKeys: redactedKeys}
}

func (s *IssueService) GetIssue(ctx context.Context, key string) (*model.Issue, error) {
	issue, err := s.db.GetIssueByKey(key)
	if errors.Is(err, model.ErrIssueRemoved) {
		return nil, err
	}
	if issue != nil {
		return issue, nil
	}
	if !errors.Is(err, model.ErrIssueNotStored) {
		log.Printf("[ERROR] GetIssueByKey %s: %s", key, err)
	}

	issue, err = s.fetchIssue(ctx, key)
	if err != nil {
		return nil, err
	}

	if !issue.Partial {
		err = s.db.UpdateIssue(ctx, issue)
		if err != nil {
			log.Printf("Error inserting issue %s: %v", key, err)
		}
	}

	return issue, nil
}

func (s *IssueService) RefreshIssue(ctx context.Context, key string) (*model.Issue, error) {
	oldIssue, _ := s.db.GetIssueForSync(key)
	if oldIssue != nil && oldIssue.IsUpToDate() {
		return nil, nil
	}

	issue, err := s.fetchIssue(ctx, key)
	if err != nil {
		if oldIssue != nil && errors.Is(err, model.ErrIssueNotFound) {
			s.db.MarkIssueRemoved(key)
			return nil, model.ErrIssueRemoved
		}
		return oldIssue, err
	}

	if issue.Partial {
		return oldIssue, errors.New("cannot refresh issue")
	}

	err = s.db.UpdateIssue(ctx, issue)
	if err != nil {
		return issue, err
	}
	return issue, nil
}

func (s *IssueService) fetchIssue(ctx context.Context, key string) (*model.Issue, error) {
	var pubIssue *api.PublicIssue
	var sdIssue *api.ServiceDeskIssue
	var pubErr, sdErr error

	done := make(chan struct{}, 2)
	go func() {
		pubIssue, pubErr = s.public.GetIssue(ctx, key)
		done <- struct{}{}
	}()
	go func() {
		sdIssue, sdErr = s.serviceDesk.GetIssue(ctx, key)
		done <- struct{}{}
	}()
	<-done
	<-done

	if sdErr != nil {
		return nil, sdErr
	}

	// Start by copying over the data from the servicedesk
	merged := model.Issue{
		Key:              key,
		Summary:          sdIssue.Summary,
		ReporterName:     sdIssue.ReporterName,
		ReporterAvatar:   sdIssue.ReporterAvatar,
		AssigneeName:     sdIssue.AssigneeName,
		AssigneeAvatar:   sdIssue.AssigneeAvatar,
		Description:      sdIssue.Description,
		Environment:      sdIssue.Environment,
		CreatedDate:      sdIssue.CreatedDate,
		Status:           sdIssue.Status,
		AffectedVersions: sdIssue.AffectedVersions,
		Components:       sdIssue.Components,
		RealmsPlatform:   sdIssue.RealmsPlatform,
		Comments:         sdIssue.Comments,
	}

	if pubErr != nil {
		merged.Partial = true
	}
	if pubIssue != nil {
		merged.Labels = pubIssue.Labels
		merged.UpdatedDate = pubIssue.UpdatedDate
		merged.ResolvedDate = pubIssue.ResolvedDate
		merged.ConfirmationStatus = pubIssue.ConfirmationStatus
		merged.Resolution = pubIssue.Resolution
		merged.FixVersions = pubIssue.FixVersions
		merged.Category = pubIssue.Category
		merged.MojangPriority = pubIssue.MojangPriority
		merged.Area = pubIssue.Area
		merged.Platform = pubIssue.Platform
		merged.OSVersion = pubIssue.OSVersion
		merged.ADO = pubIssue.ADO
		merged.Votes = pubIssue.Votes
		merged.Links = pubIssue.Links
		merged.Attachments = pubIssue.Attachments
		now := time.Now()
		merged.SyncedDate = &now
	}

	for i := range merged.Comments {
		merged.Comments[i].Issue = &merged
	}

	return &merged, nil
}
