package main

import (
	"context"
	"fmt"
	"log"
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
			queueUpdatedIssues(service)
		}
	}()

	go func() {
		ticker := time.NewTicker(15 * time.Second)
		for {
			<-ticker.C
			processQueuedIssues(service)
		}
	}()

	go func() {
		ticker := time.NewTicker(5 * time.Second)
		for {
			<-ticker.C
			runInitialSync(service)
		}
	}()
}

func queueUpdatedIssues(service *IssueService) {
	t0 := time.Now()
	ctx := context.Background()
	updatedKeys, err := service.serviceDesk.GetUpdatedIssues(ctx)
	if err != nil {
		log.Printf("[queue] Error fetching updated issues: %v\n", err)
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
		log.Printf("[queue] Error queuing issues: %v\n", err)
		return
	}
	t1 := time.Now()
	log.Printf("[queue] Queued %d updated issues in %s: %s\n", len(queuedKeys), t1.Sub(t0), strings.Join(queuedKeys, ", "))
}

func processQueuedIssues(service *IssueService) {
	ctx := context.Background()
	keys, err := service.db.GetQueuedIssueKeys(ctx, 10)
	if err != nil {
		log.Printf("[queue] Error fetching issue keys: %v\n", err)
		return
	}
	if len(keys) == 0 {
		return
	}
	for _, key := range keys {
		_, err := service.RefreshIssue(ctx, key)
		if err != nil {
			log.Printf("[queue] Error refreshing issue %s: %v\n", key, err)
			service.db.ReQueueIssueKey(ctx, key)
			continue
		}
		err = service.db.RemoveQueuedIssueKey(ctx, key)
		if err != nil {
			log.Printf("[queue] Error removing key %s: %v\n", key, err)
		}
	}
}

func runInitialSync(service *IssueService) {
	t0 := time.Now()
	count := 0
	ctx := context.Background()
	for _, prefix := range projects {
		max, err := service.db.GetMaxIssueNumberForPrefix(ctx, prefix)
		if err != nil {
			log.Printf("[sync] Error getting max issue number for %s: %v\n", prefix, err)
			continue
		}
		last, err := service.db.GetSyncState(ctx, prefix)
		if err != nil {
			log.Printf("[sync] Error getting sync state for %s: %v\n", prefix, err)
			continue
		}
		batchSize := 10
		start := last + 1
		end := min(start+batchSize-1, max)
		for i := start; i <= end; i++ {
			key := fmt.Sprintf("%s-%d", prefix, i)
			_, err := service.RefreshIssue(ctx, key)
			if err != nil {
				err = service.db.InsertUnknownIssue(key)
				if err != nil {
					log.Printf("[sync] Error inserting unknown issue %s: %v\n", key, err)
					break
				}
			}
			count += 1
			err = service.db.SetSyncState(ctx, prefix, i)
			if err != nil {
				log.Printf("[sync] Error updating sync state for %s: %v\n", prefix, err)
				break
			}
		}
	}
	t1 := time.Now()
	if count > 0 {
		log.Printf("[sync] Initial sync %v in %s\n", count, t1.Sub(t0))
	}
}
