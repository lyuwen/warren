package web

import (
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		if origin == "" {
			// Same-origin request (no Origin header)
			return true
		}

		// Parse origin URL
		u, err := url.Parse(origin)
		if err != nil {
			log.Printf("Invalid origin URL: %v", err)
			return false
		}

		// Only allow localhost origins
		allowedHosts := []string{
			"localhost:8080",
			"127.0.0.1:8080",
			"localhost",
			"127.0.0.1",
		}

		for _, host := range allowedHosts {
			if u.Host == host {
				return true
			}
		}

		log.Printf("Rejected WebSocket connection from origin: %s", origin)
		return false
	},
}

// Hub maintains active WebSocket connections and broadcasts messages
type Hub struct {
	clients    map[*Client]bool
	broadcast  chan interface{}
	register   chan *Client
	unregister chan *Client
	mu         sync.RWMutex
	running    bool
}

// Client represents a WebSocket client connection
type Client struct {
	hub  *Hub
	conn *websocket.Conn
	send chan interface{}
}

// NewHub creates a new WebSocket hub
func NewHub() *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
		broadcast:  make(chan interface{}, 256),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		running:    false,
	}
}

// Run starts the hub's main loop
func (h *Hub) Run() {
	h.mu.Lock()
	h.running = true
	h.mu.Unlock()

	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()
			log.Printf("WebSocket client connected (total: %d)", len(h.clients))

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
			h.mu.Unlock()
			log.Printf("WebSocket client disconnected (total: %d)", len(h.clients))

		case message := <-h.broadcast:
			h.mu.RLock()
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					// Client's send buffer is full, disconnect
					close(client.send)
					delete(h.clients, client)
				}
			}
			h.mu.RUnlock()
		}
	}
}

// Stop stops the hub
func (h *Hub) Stop() {
	h.mu.Lock()
	defer h.mu.Unlock()

	if !h.running {
		return
	}

	h.running = false

	// Close all client connections
	for client := range h.clients {
		close(client.send)
		client.conn.Close()
	}

	h.clients = make(map[*Client]bool)
}

// Broadcast sends a message to all connected clients
func (h *Hub) Broadcast(message interface{}) {
	select {
	case h.broadcast <- message:
	default:
		// Broadcast channel full, drop message
	}
}

// handleWebSocket handles WebSocket upgrade and client lifecycle
func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}

	client := &Client{
		hub:  s.wsHub,
		conn: conn,
		send: make(chan interface{}, 256),
	}

	client.hub.register <- client

	// Start goroutines for reading and writing
	go client.writePump()
	go client.readPump()
}

// readPump reads messages from the WebSocket connection
func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, _, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket read error: %v", err)
			}
			break
		}
	}
}

// writePump writes messages to the WebSocket connection
func (c *Client) writePump() {
	ticker := time.NewTicker(54 * time.Second)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				// Hub closed the channel
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			// Send JSON message
			data, err := json.Marshal(message)
			if err != nil {
				log.Printf("JSON marshal error: %v", err)
				continue
			}

			if err := c.conn.WriteMessage(websocket.TextMessage, data); err != nil {
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
