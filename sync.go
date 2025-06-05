package main

import (
	"context"
	"errors"
	"log"
	"mojira/model"
	"slices"
	"strings"
	"time"
)

var projects = []string{"MC", "MCPE", "MCL", "REALMS", "WEB", "BDS"}

func StartSync(service *IssueService, noSync bool) {
	if !noSync {
		log.Println("Starting update feed listener...")
		go func() {
			ticker := time.NewTicker(10 * time.Second)
			for {
				<-ticker.C
				updateListener(service)
			}
		}()
	}

	log.Println("Starting queue processor...")
	go func() {
		ticker := time.NewTicker(4 * time.Second)
		for {
			<-ticker.C
			queueProcessor(service)
		}
	}()
}

func updateListener(service *IssueService) {
	t0 := time.Now()
	ctx := context.Background()
	updatedKeys, err := service.serviceDesk.GetUpdatedIssues(ctx)
	if err != nil {
		log.Printf("[listener] Error fetching issues: %v", err)
		return
	}
	var filteredKeys []string
	for _, key := range updatedKeys {
		if slices.Contains(projects, strings.Split(key, "-")[0]) {
			filteredKeys = append(filteredKeys, key)
		}
	}
	queuedKeys, err := service.db.QueueIssueKeys(filteredKeys, 10, "update-feed")
	if err != nil {
		log.Printf("[listener] Error queueing issues: %v", err)
		return
	}
	t1 := time.Now()
	if len(queuedKeys) > 0 {
		log.Printf("[listener] Queued %d issues (%s): %s", len(queuedKeys), t1.Sub(t0), strings.Join(queuedKeys, ", "))
	}
}

func queueProcessor(service *IssueService) {
	ctx := context.Background()
	rows, err := service.db.PopQueuedIssues(ctx, 10)
	if err != nil {
		log.Printf("[queue] Error getting queued keys: %v", err)
		return
	}
	for _, row := range rows {
		_, err := service.RefreshIssue(ctx, row.Key)
		if err != nil {
			if errors.Is(err, model.ErrIssueRemoved) {
				log.Printf("[queue] Detected removed issue %s", row.Key)
				service.db.MarkIssueRemoved(row.Key)
			} else {
				service.db.RetryQueuedIssue(ctx, row)
			}
		} else {
			log.Printf("[queue] Refreshed issue %s", row.Key)
		}
	}
}
