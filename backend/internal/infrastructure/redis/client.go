// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

// Client est le wrapper OpenRisk autour de go-redis.
// Il expose uniquement les méthodes utilisées dans le projet.
type Client struct {
	redis *goredis.Client
}

// NewClient initialise le client Redis depuis les variables d'env.
// Env vars:
//   REDIS_URL (ex: redis://localhost:6379) — obligatoire
//   REDIS_PASSWORD (optionnel)
//   REDIS_DB (optionnel, défaut: 0)
// Teste la connexion au démarrage (Ping).
// Panic si connexion impossible (fail-fast au boot, pas en runtime).
func NewClient(redisURL string) *Client {
	opts, err := goredis.ParseURL(redisURL)
	if err != nil {
		panic(fmt.Sprintf("failed to parse REDIS_URL: %v", err))
	}

	redisClient := goredis.NewClient(opts)

	// Test connection immediately
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := redisClient.Ping(ctx).Err(); err != nil {
		panic(fmt.Sprintf("failed to connect to Redis: %v", err))
	}

	return &Client{redis: redisClient}
}

// Publish publie un message sur un canal Redis.
// Le payload est automatiquement JSON-marshaled.
func (c *Client) Publish(ctx context.Context, channel string, payload interface{}) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	cmd := c.redis.Publish(ctx, channel, string(data))
	if err := cmd.Err(); err != nil {
		return fmt.Errorf("failed to publish to channel %s: %w", channel, err)
	}

	return nil
}

// Subscribe s'abonne à un ou plusieurs canaux Redis.
// Retourne un PubSub pour lire les messages.
func (c *Client) Subscribe(ctx context.Context, channels ...string) *goredis.PubSub {
	return c.redis.Subscribe(ctx, channels...)
}

// Set stocke une clé-valeur avec TTL expiration.
func (c *Client) Set(ctx context.Context, key string, value string, ttl time.Duration) error {
	cmd := c.redis.Set(ctx, key, value, ttl)
	if err := cmd.Err(); err != nil {
		return fmt.Errorf("failed to set key %s: %w", key, err)
	}
	return nil
}

// Get récupère une valeur par clé.
func (c *Client) Get(ctx context.Context, key string) (string, error) {
	cmd := c.redis.Get(ctx, key)
	val, err := cmd.Result()
	if err != nil {
		if err == goredis.Nil {
			return "", nil // Key not found, return empty string
		}
		return "", fmt.Errorf("failed to get key %s: %w", key, err)
	}
	return val, nil
}

// Del supprime une ou plusieurs clés.
func (c *Client) Del(ctx context.Context, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}
	cmd := c.redis.Del(ctx, keys...)
	if err := cmd.Err(); err != nil {
		return fmt.Errorf("failed to delete keys: %w", err)
	}
	return nil
}

// Exists vérifie si une clé existe.
func (c *Client) Exists(ctx context.Context, key string) (bool, error) {
	cmd := c.redis.Exists(ctx, key)
	val, err := cmd.Result()
	if err != nil {
		return false, fmt.Errorf("failed to check key existence: %w", err)
	}
	return val > 0, nil
}

// Incr incrémente une valeur numérique.
func (c *Client) Incr(ctx context.Context, key string) (int64, error) {
	cmd := c.redis.Incr(ctx, key)
	val, err := cmd.Result()
	if err != nil {
		return 0, fmt.Errorf("failed to increment key %s: %w", key, err)
	}
	return val, nil
}

// Expire défini l'expiration d'une clé.
func (c *Client) Expire(ctx context.Context, key string, ttl time.Duration) error {
	cmd := c.redis.Expire(ctx, key, ttl)
	if err := cmd.Err(); err != nil {
		return fmt.Errorf("failed to expire key %s: %w", key, err)
	}
	return nil
}

// Close ferme la connexion Redis.
func (c *Client) Close() error {
	return c.redis.Close()
}

// Ping teste la connexion Redis.
func (c *Client) Ping(ctx context.Context) error {
	cmd := c.redis.Ping(ctx)
	return cmd.Err()
}
