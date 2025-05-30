package main

import (
	"context"
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

	ctx := context.Background()
	serviceDeskClient, err := api.NewServiceDeskClient(ctx)
	if err != nil {
		log.Fatal(err)
	}

	publicClient := api.NewPublicClient()

	return &IssueService{db: dbClient, publicAPI: *publicClient, serviceDesk: *serviceDeskClient}
}

func (s *IssueService) GetIssue(ctx context.Context, key string) (*model.Issue, error) {
	issue, _ := s.db.GetIssueByKey(key)
	if issue != nil {
		return issue, nil
	}

	issue, err := s.FetchIssue(ctx, key)
	if err != nil {
		return nil, err
	}

	err = s.db.InsertIssue(ctx, issue)
	if err != nil {
		log.Printf("Error inserting issue %s: %v", key, err)
	}

	return issue, nil
}

func (s *IssueService) FetchIssue(ctx context.Context, key string) (*model.Issue, error) {
	var pubIssue *model.Issue
	var sdIssue *model.Issue
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

	if pubErr != nil {
		return nil, pubErr
	}
	if sdErr != nil {
		return nil, sdErr
	}

	merged := model.Issue{Key: key}
	if pubIssue != nil {
		merged = *pubIssue
	}
	if sdIssue != nil {
		if merged.Summary == "" {
			merged.Summary = sdIssue.Summary
		}
		if merged.ReporterName == "" {
			merged.ReporterName = sdIssue.ReporterName
		}
		if merged.ReporterAvatar == "" {
			merged.ReporterAvatar = sdIssue.ReporterAvatar
		}
		if merged.Description == "" {
			merged.Description = sdIssue.Description
		}
		if merged.Environment == "" {
			merged.Environment = sdIssue.Environment
		}
		if merged.CreatedDate == nil {
			merged.CreatedDate = sdIssue.CreatedDate
		}
		if merged.Status == "" {
			merged.Status = sdIssue.Status
		}
		if merged.AffectedVersions == "" {
			merged.AffectedVersions = sdIssue.AffectedVersions
		}
		if merged.Comments == nil {
			merged.Comments = sdIssue.Comments
		}
	}

	return &merged, nil
}
