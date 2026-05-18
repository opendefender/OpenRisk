package redis

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/redis/go-redis/v9"
	"github.com/opendefender/openrisk/internal/domain"
)

// MitigationEventsPublisher publishes mitigation-related events to Redis
type MitigationEventsPublisher struct {
	client *redis.Client
}

func NewMitigationEventsPublisher(client *redis.Client) *MitigationEventsPublisher {
	return &MitigationEventsPublisher{client: client}
}

// PublishMitigationProgressChanged publishes when progress changes
func (p *MitigationEventsPublisher) PublishMitigationProgressChanged(ctx context.Context, evt *domain.MitigationProgressChanged) error {
	data, err := json.Marshal(evt)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	err = p.client.Publish(ctx, "mitigation.progress_changed", string(data)).Err()
	if err != nil {
		return fmt.Errorf("failed to publish mitigation.progress_changed: %w", err)
	}

	return nil
}

// PublishMitigationCompleted publishes when plan is fully completed
func (p *MitigationEventsPublisher) PublishMitigationCompleted(ctx context.Context, evt *domain.MitigationCompleted) error {
	data, err := json.Marshal(evt)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	err = p.client.Publish(ctx, "mitigation.completed", string(data)).Err()
	if err != nil {
		return fmt.Errorf("failed to publish mitigation.completed: %w", err)
	}

	return nil
}

// PublishMitigationAutoCompleted publishes when scanner auto-completes a subaction
func (p *MitigationEventsPublisher) PublishMitigationAutoCompleted(ctx context.Context, evt *domain.MitigationAutoCompleted) error {
	data, err := json.Marshal(evt)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	err = p.client.Publish(ctx, "mitigation.auto_completed", string(data)).Err()
	if err != nil {
		return fmt.Errorf("failed to publish mitigation.auto_completed: %w", err)
	}

	return nil
}

// PublishMitigationReverted publishes when a subaction is reverted
func (p *MitigationEventsPublisher) PublishMitigationReverted(ctx context.Context, evt *domain.MitigationReverted) error {
	data, err := json.Marshal(evt)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	err = p.client.Publish(ctx, "mitigation.reverted", string(data)).Err()
	if err != nil {
		return fmt.Errorf("failed to publish mitigation.reverted: %w", err)
	}

	return nil
}
