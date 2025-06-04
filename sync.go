package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"mojira/model"
	"slices"
	"strings"
	"time"
)

var projects = []string{"MC", "MCPE", "MCL", "REALMS", "WEB", "BDS"}

func StartSync(service *IssueService) {
	log.Println("Starting sync tickers...")

	go func() {
		ticker := time.NewTicker(10 * time.Second)
		for {
			<-ticker.C
			updateListener(service)
		}
	}()

	go func() {
		ticker := time.NewTicker(17 * time.Second)
		for {
			<-ticker.C
			queueProcessor(service)
		}
	}()

	go func() {
		ticker := time.NewTicker(6 * time.Second)
		for {
			<-ticker.C
			issueScanner(service)
		}
	}()
}

func updateListener(service *IssueService) {
	t0 := time.Now()
	ctx := context.Background()
	updatedKeys, err := service.serviceDesk.GetUpdatedIssues(ctx)
	if err != nil {
		log.Printf("[updateListener] Error fetching issues: %v", err)
		return
	}
	var filteredKeys []string
	for _, key := range updatedKeys {
		if slices.Contains(projects, strings.Split(key, "-")[0]) {
			filteredKeys = append(filteredKeys, key)
		}
	}
	queuedKeys, err := service.db.QueueIssueKeys(filteredKeys)
	if err != nil {
		log.Printf("[updateListener] Error queueing issues: %v", err)
		return
	}
	t1 := time.Now()
	if len(queuedKeys) > 0 {
		log.Printf("[updateListener] Queued %d issues (%s): %s", len(queuedKeys), t1.Sub(t0), strings.Join(queuedKeys, ", "))
	}
}

func queueProcessor(service *IssueService) {
	ctx := context.Background()
	keys, err := service.db.GetQueuedIssueKeys(ctx, 10)
	if err != nil {
		log.Printf("[queueProcessor] Error getting queued keys: %v", err)
		return
	}
	for _, key := range keys {
		_, err := service.RefreshIssue(ctx, key)
		if err != nil {
			if errors.Is(err, model.ErrIssueRemoved) {
				log.Printf("[queueProcessor] Detected removed issue %s", key)
				service.db.MarkIssueRemoved(key)
			} else {
				service.db.ReQueueIssueKey(ctx, key)
				continue
			}
		} else {
			log.Printf("[queueProcessor] Refreshed issue %s", key)
		}
		err = service.db.RemoveQueuedIssueKey(ctx, key)
		if err != nil {
			log.Printf("[queueProcessor] Error removing queued key %s: %v", key, err)
		}
	}
}

func issueScanner(service *IssueService) {
	t0 := time.Now()
	count := 0
	ctx := context.Background()
	for _, prefix := range projects {
		max, err := service.db.GetMaxIssueNumberForPrefix(ctx, prefix)
		if err != nil {
			log.Printf("[issueScanner] Error getting max issue number for %s: %v", prefix, err)
			continue
		}
		last, err := service.db.GetSyncState(ctx, prefix)
		if err != nil {
			log.Printf("[issueScanner] Error getting sync state for %s: %v", prefix, err)
			continue
		}
		batchSize := 10
		start := last + 1
		end := min(start+batchSize-1, max)
		for i := start; i <= end; i++ {
			key := fmt.Sprintf("%s-%d", prefix, i)
			_, err := service.RefreshIssue(ctx, key)
			if err != nil {
				log.Printf("[issueScanner] Error when refreshing %s: %v", prefix, err)
			} else {
				count += 1
			}
			err = service.db.SetSyncState(ctx, prefix, i)
			if err != nil {
				log.Printf("[issueScanner] Error updating sync state for %s: %v", prefix, err)
				break
			}
		}
	}
	t1 := time.Now()
	if count > 0 {
		log.Printf("[issueScanner] Synced %v issues (%s)", count, t1.Sub(t0))
	}
}
