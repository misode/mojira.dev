package main

import (
	"context"
	"errors"
	"log"
	"mojira/api"
	"mojira/model"
	"time"
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

	err = s.db.InsertIssue(ctx, issue)
	if err != nil {
		log.Printf("Error inserting issue %s: %v", key, err)
	}

	return issue, nil
}

func (s *IssueService) RefreshIssue(ctx context.Context, key string) (*model.Issue, error) {
	oldIssue, _ := s.db.GetIssueByKey(key)
	if oldIssue != nil && oldIssue.IsUpToDate() {
		return nil, nil
	}
	issue, err := s.fetchIssue(ctx, key)
	if err != nil {
		if oldIssue == nil {
			return nil, err
		}
		err = s.db.MarkIssueRemoved(key)
		if err != nil {
			return nil, err
		}
		return nil, model.ErrIssueRemoved
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
		if merged.AssigneeName == "" {
			merged.AssigneeName = sdIssue.AssigneeName
		}
		if merged.AssigneeAvatar == "" {
			merged.AssigneeAvatar = sdIssue.AssigneeAvatar
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
		if len(merged.AffectedVersions) == 0 {
			merged.AffectedVersions = sdIssue.AffectedVersions
		}
		if len(merged.Components) == 0 {
			merged.Components = sdIssue.Components
		}
		if merged.RealmsPlatform == "" {
			merged.RealmsPlatform = sdIssue.RealmsPlatform
		}
		if merged.Comments == nil {
			merged.Comments = sdIssue.Comments
		}
	}

	synced := time.Now()
	merged.SyncedDate = &synced

	return &merged, nil
}
