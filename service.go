package main

import (
	"context"
	"errors"
	"log"
	"mojira/api"
	"mojira/model"
)

type IssueService struct {
	db          *DBClient
	publicAPI   api.PublicClient
	serviceDesk api.ServiceDeskClient
}

func NewIssueService() *IssueService {
	dbClient, err := NewDBClient()
	if err != nil {
		log.Fatal(err)
	}

	pubClient := api.NewPublicClient()

	ctx := context.Background()
	sdClient, err := api.NewServiceDeskClient(ctx)
	if err != nil {
		log.Fatal(err)
	}

	return &IssueService{db: dbClient, publicAPI: *pubClient, serviceDesk: *sdClient}
}

func (s *IssueService) GetIssue(ctx context.Context, key string) (*model.Issue, error) {
	issue, err := s.db.GetIssueByKey(key)
	if errors.Is(err, model.ErrIssueRemoved) {
		return nil, err
	}
	if issue != nil {
		return issue, nil
	}

	issue, err = s.fetchIssue(ctx, key)
	if err != nil {
		return nil, err
	}

	if !issue.Partial {
		err = s.db.InsertIssue(ctx, issue)
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
		if oldIssue == nil {
			return nil, err
		}
		s.db.MarkIssueRemoved(key)
		return nil, model.ErrIssueRemoved
	}

	if issue.Partial {
		return nil, errors.New("cannot refresh issue")
	}

	err = s.db.InsertIssue(ctx, issue)
	if err != nil {
		log.Printf("Error inserting refreshed issue %s: %v", key, err)
	} else {
		log.Printf("Refreshed issue %s", key)
	}
	return issue, nil
}

func (s *IssueService) fetchIssue(ctx context.Context, key string) (*model.Issue, error) {
	var pubIssue *api.PublicIssue
	var sdIssue *api.ServiceDeskIssue
	var pubErr, sdErr error

	done := make(chan struct{}, 2)

	go func() {
		pubIssue, pubErr = s.publicAPI.GetIssue(ctx, key)
		done <- struct{}{}
	}()
	go func() {
		sdIssue, sdErr = s.serviceDesk.GetIssue(ctx, key)
		done <- struct{}{}
	}()
	<-done
	<-done

	if sdErr != nil {
		// Servicedesk API error, assume issue was removed
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
		merged.MojangPriority = pubIssue.MojangPriority
		merged.Area = pubIssue.Area
		merged.Platform = pubIssue.Platform
		merged.OSVersion = pubIssue.OSVersion
		merged.ADO = pubIssue.ADO
		merged.Votes = pubIssue.Votes
		merged.Links = pubIssue.Links
		merged.Attachments = pubIssue.Attachments
	}

	return &merged, nil
}
