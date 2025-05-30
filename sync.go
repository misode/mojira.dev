package main

import (
	"context"
	"fmt"
	"time"
)

func StartSync(service *IssueService) {
	fmt.Println("Starting sync tickers...")

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
		fmt.Printf("Error fetching updated issues: %v\n", err)
		return
	}
	count, err := service.db.QueueIssueKeys(updatedKeys)
	if err != nil {
		fmt.Printf("Error queuing updated issues: %v\n", err)
		return
	}
	t1 := time.Now()
	fmt.Printf("Queued %d updated issues (%s)\n", count, t1.Sub(t0))
}

func processQueuedIssues(service *IssueService) {
	ctx := context.Background()
	keys, err := service.db.GetQueuedIssueKeys(ctx, 10)
	if err != nil {
		fmt.Printf("Error fetching queued issue keys: %v\n", err)
		return
	}
	if len(keys) == 0 {
		return
	}
	for _, key := range keys {
		_, err := service.GetIssue(ctx, key)
		if err != nil {
			fmt.Printf("Error fetching issue %s: %v\n", key, err)
			continue
		}
		err = service.db.RemoveQueuedIssueKey(ctx, key)
		if err != nil {
			fmt.Printf("Error removing queued key %s: %v\n", key, err)
		}
	}
}

func runInitialSync(service *IssueService) {
	t0 := time.Now()
	prefixes := []string{"MC", "MCPE", "MCL", "REALMS", "WEB", "BDS"}
	count := 0
	ctx := context.Background()
	for _, prefix := range prefixes {
		max, err := service.db.GetMaxIssueNumberForPrefix(ctx, prefix)
		if err != nil {
			fmt.Printf("Error getting max issue number for %s: %v\n", prefix, err)
			continue
		}
		last, err := service.db.GetSyncState(ctx, prefix)
		if err != nil {
			fmt.Printf("Error getting sync state for %s: %v\n", prefix, err)
			continue
		}
		batchSize := 5
		start := last + 1
		end := start + batchSize - 1
		if end > max {
			end = max
		}
		for i := start; i <= end; i++ {
			key := fmt.Sprintf("%s-%d", prefix, i)
			issue, err := service.FetchIssue(ctx, key)
			if err != nil {
				err = service.db.InsertMissingIssue(key)
				if err != nil {
					fmt.Printf("Error inserting missing issue %s: %v\n", key, err)
					break
				}
			} else {
				err = service.db.InsertIssue(ctx, issue)
				if err != nil {
					fmt.Printf("Error inserting issue %s: %v", key, err)
					break
				}
			}
			count += 1
			err = service.db.SetSyncState(ctx, prefix, i)
			if err != nil {
				fmt.Printf("Error updating sync state for %s: %v\n", prefix, err)
				break
			}
		}
	}
	t1 := time.Now()
	fmt.Printf("Initial sync %v in %s\n", count, t1.Sub(t0))
}
