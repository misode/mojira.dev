package main

import (
	"context"
	"errors"
	"log"
	"mojira/model"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var projects = []string{"MC", "MCPE", "MCL", "REALMS", "WEB", "BDS"}

var syncQueueCount = promauto.NewGauge(
	prometheus.GaugeOpts{
		Name: "mojira_sync_queue_size",
		Help: "Number of rows in sync_queue table",
	},
)

func StartSync(service *IssueService, noSync bool) {
	updateMetric(service, context.Background())

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
		ticker := time.NewTicker(3 * time.Second)
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
	keys, err := service.db.PeekQueuedIssues(ctx, 10)
	if err != nil {
		log.Printf("[ERROR] [queue] Error getting queued keys: %v", err)
		return
	}
	var wg sync.WaitGroup
	for _, key := range keys {
		wg.Add(1)
		go func(key string) {
			defer wg.Done()
			_, err := service.RefreshIssue(ctx, key)
			if err != nil {
				if errors.Is(err, model.ErrIssueRemoved) {
					log.Printf("[queue] Detected removed issue %s", key)
					service.db.MarkIssueRemoved(key)
				} else {
					err = service.db.RetryQueuedIssue(ctx, key)
					if err != nil {
						log.Printf("[ERROR] [queue] Error retrying queued issue %s: %v", key, err)
					}
					return
				}
			} else {
				log.Printf("[queue] Refreshed issue %s", key)
			}
			err = service.db.DeleteQueuedIssue(ctx, key)
			if err != nil {
				log.Printf("[ERROR] [queue] Error deleting queued issue %s: %v", key, err)
			}
		}(key)
	}
	wg.Wait()
	updateMetric(service, ctx)
}

func updateMetric(service *IssueService, ctx context.Context) {
	count, err := service.db.GetQueueSize(ctx)
	if err != nil {
		log.Printf("[ERROR] [queue] Error getting queue size: %v", err)
	}
	syncQueueCount.Set(float64(count))
}
