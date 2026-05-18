package workers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/redis/go-redis/v9"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/infrastructure/database"
	"github.com/opendefender/openrisk/internal/infrastructure/repository"
)

// MitigationEventWorker listens to mitigation events and triggers score recalculation
type MitigationEventWorker struct {
	redisClient  *redis.Client
	riskRepo     repository.RiskRepository
}

func NewMitigationEventWorker(redisClient *redis.Client, riskRepo repository.RiskRepository) *MitigationEventWorker {
	return &MitigationEventWorker{
		redisClient: redisClient,
		riskRepo:    riskRepo,
	}
}

// Start begins listening to mitigation events
func (w *MitigationEventWorker) Start(ctx context.Context) error {
	pubsub := w.redisClient.Subscribe(ctx, 
		"mitigation.progress_changed",
		"mitigation.completed",
		"mitigation.auto_completed",
		"mitigation.reverted",
	)
	defer pubsub.Close()

	ch := pubsub.Channel()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case msg := <-ch:
			if msg == nil {
				return fmt.Errorf("received nil message")
			}

			switch msg.Channel {
			case "mitigation.progress_changed":
				w.handleProgressChanged(ctx, msg.Payload)
			case "mitigation.completed":
				w.handleMitigationCompleted(ctx, msg.Payload)
			case "mitigation.auto_completed":
				w.handleAutoCompleted(ctx, msg.Payload)
			case "mitigation.reverted":
				w.handleReverted(ctx, msg.Payload)
			}
		}
	}
}

// handleProgressChanged updates UI, checks if auto-transitioning to review
func (w *MitigationEventWorker) handleProgressChanged(ctx context.Context, payload string) {
	var evt domain.MitigationProgressChanged
	if err := json.Unmarshal([]byte(payload), &evt); err != nil {
		log.Printf("ERROR: failed to unmarshal progress changed event: %v", err)
		return
	}

	log.Printf("Mitigation %s progress changed to %d, status: %s", evt.PlanID, evt.Progress, evt.Status)
	// Event already published by handlers, just log it
}

// handleMitigationCompleted triggers score engine recalculation for the risk
func (w *MitigationEventWorker) handleMitigationCompleted(ctx context.Context, payload string) {
	var evt domain.MitigationCompleted
	if err := json.Unmarshal([]byte(payload), &evt); err != nil {
		log.Printf("ERROR: failed to unmarshal mitigation completed event: %v", err)
		return
	}

	log.Printf("Mitigation %s completed for risk %s (source: %s)", evt.PlanID, evt.RiskID, evt.Source)

	// TODO: Trigger Score Engine recalculation
	// This would involve:
	// 1. Getting the risk
	// 2. Calling Score Engine to recalculate
	// 3. Checking if score < 2.0 to auto-transition risk to "mitigated"
	// 4. Publishing risk.score_changed event

	// For now, just log
}

// handleAutoCompleted logs scanner auto-completion
func (w *MitigationEventWorker) handleAutoCompleted(ctx context.Context, payload string) {
	var evt domain.MitigationAutoCompleted
	if err := json.Unmarshal([]byte(payload), &evt); err != nil {
		log.Printf("ERROR: failed to unmarshal auto completed event: %v", err)
		return
	}

	log.Printf("Subaction %s auto-completed by scanner job %s", evt.SubActionID, evt.ScannerJobID)
	// Event already published by handlers
}

// handleReverted logs subaction revert
func (w *MitigationEventWorker) handleReverted(ctx context.Context, payload string) {
	var evt domain.MitigationReverted
	if err := json.Unmarshal([]byte(payload), &evt); err != nil {
		log.Printf("ERROR: failed to unmarshal reverted event: %v", err)
		return
	}

	log.Printf("Subaction %s reverted by user %s", evt.SubActionID, evt.RevertedBy)
	// Score engine recalculation might be needed if mitigation was previously completed
}
