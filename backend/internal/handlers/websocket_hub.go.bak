package handlers

import (
	"context"
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/opendefender/openrisk/internal/services"
)

// WebSocketHub manages WebSocket connections and broadcasts
type WebSocketHub struct {
	clients      map[*websocket.Conn]bool
	broadcast    chan interface{}
	register     chan *websocket.Conn
	unregister   chan *websocket.Conn
	mu           sync.RWMutex
	ticker       *time.Ticker
	dashService  *services.DashboardDataService
	tickInterval time.Duration
}

// NewWebSocketHub creates a new WebSocket hub
func NewWebSocketHub(dashService *services.DashboardDataService, tickInterval time.Duration) *WebSocketHub {
	return &WebSocketHub{
		clients:      make(map[*websocket.Conn]bool),
		broadcast:    make(chan interface{}, 256),
		register:     make(chan *websocket.Conn),
		unregister:   make(chan *websocket.Conn),
		dashService:  dashService,
		tickInterval: tickInterval,
	}
}

// Run starts the WebSocket hub
func (h *WebSocketHub) Run(ctx context.Context) {
	h.ticker = time.NewTicker(h.tickInterval)
	defer h.ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			h.shutdown()
			return

		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()
			log.Printf("WebSocket client connected. Total clients: %d", len(h.clients))

		case client := <-h.unregister:
			h.mu.Lock()
			if ok := h.clients[client]; ok {
				delete(h.clients, client)
				client.Close()
			}
			h.mu.Unlock()
			log.Printf("WebSocket client disconnected. Total clients: %d", len(h.clients))

		case message := <-h.broadcast:
			h.broadcastMessage(message)

		case <-h.ticker.C:
			// Fetch fresh data and broadcast
			h.fetchAndBroadcastData(ctx)
		}
	}
}

// HandleWebSocket handles WebSocket connections
func (h *WebSocketHub) HandleWebSocket(c *websocket.Conn) {
	client := c
	h.register <- client
	defer func() {
		h.unregister <- client
	}()

	// Send initial data
	h.fetchAndBroadcastToClient(c, c.Context())

	// Read messages from client (for subscription control, etc.)
	for {
		mt, msg, err := client.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}

		// Handle client commands
		if mt == websocket.TextMessage {
			var command map[string]interface{}
			if err := json.Unmarshal(msg, &command); err == nil {
				h.handleClientCommand(c, command)
			}
		}
	}
}

// handleClientCommand processes commands from client
func (h *WebSocketHub) handleClientCommand(c *websocket.Conn, command map[string]interface{}) {
	if action, ok := command["action"].(string); ok {
		switch action {
		case "refresh":
			// Client requested immediate refresh
			h.fetchAndBroadcastToClient(c, c.Context())
		case "ping":
			// Respond with pong
			c.WriteJSON(map[string]string{"type": "pong"})
		}
	}
}

// fetchAndBroadcastData fetches fresh data and broadcasts to all clients
func (h *WebSocketHub) fetchAndBroadcastData(ctx context.Context) {
	analytics, err := h.dashService.GetCompleteDashboardData(ctx)
	if err != nil {
		log.Printf("Error fetching dashboard data: %v", err)
		h.broadcast <- map[string]interface{}{
			"type":  "error",
			"error": err.Error(),
		}
		return
	}

	h.broadcast <- map[string]interface{}{
		"type": "dashboard_update",
		"data": analytics,
	}
}

// fetchAndBroadcastToClient sends data to a specific client
func (h *WebSocketHub) fetchAndBroadcastToClient(c *websocket.Conn, ctx context.Context) {
	analytics, err := h.dashService.GetCompleteDashboardData(ctx)
	if err != nil {
		log.Printf("Error fetching dashboard data: %v", err)
		c.WriteJSON(map[string]interface{}{
			"type":  "error",
			"error": err.Error(),
		})
		return
	}

	c.WriteJSON(map[string]interface{}{
		"type": "dashboard_update",
		"data": analytics,
	})
}

// broadcastMessage broadcasts a message to all connected clients
func (h *WebSocketHub) broadcastMessage(message interface{}) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for client := range h.clients {
		if err := client.WriteJSON(message); err != nil {
			log.Printf("Error writing to client: %v", err)
			h.unregister <- client
		}
	}
}

// GetClientCount returns the number of connected clients
func (h *WebSocketHub) GetClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// shutdown closes all client connections
func (h *WebSocketHub) shutdown() {
	h.mu.Lock()
	defer h.mu.Unlock()

	for client := range h.clients {
		client.Close()
	}
	h.clients = make(map[*websocket.Conn]bool)
	close(h.broadcast)
}

// DashboardWebSocketMetrics returns metrics about WebSocket connections
func (h *WebSocketHub) DashboardWebSocketMetrics(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"connected_clients": h.GetClientCount(),
		"status":            "active",
	})
}
