// Copyright 2013 The Gorilla WebSocket Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 512
)

var (
	newline = []byte{'\n'}
	space   = []byte{' '}
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type Client struct {
	hub *Hub

	conn *websocket.Conn

	send chan []byte

	player *Player

	game *Game
}

func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		// Remove player from game if exists
		if c.player != nil {
			c.game.removePlayer(c.player.ID, false)
			log.Printf("Player %d disconnected", c.player.ID)
		}
		c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))

	c.conn.SetPongHandler(func(string) error { c.conn.SetReadDeadline(time.Now().Add(pongWait)); return nil })

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("error: %v", err)
			}
			break
		}
		message = bytes.TrimSpace(bytes.Replace(message, newline, space, -1))

		var data map[string]any
		err = json.Unmarshal(message, &data)
		if err != nil {
			log.Println("error unmarshalling message", err)
			continue
		}

		// Check message type
		if messageType, ok := data["type"].(string); ok {
			switch messageType {
			case "join":
				// Create a new player only if the client doesn't already have one
				if c.player == nil {
					c.player = c.game.addNewPlayer()

					// Send acknowledgment back to client
					response := map[string]any{
						"type":     "joined",
						"playerID": c.player.ID,
					}
					responseJSON, _ := json.Marshal(response)
					c.send <- responseJSON
				}

			case "direction":
				// Only update direction if player exists
				if c.player != nil && data["direction"] != nil {
					direction := data["direction"].(float64)
					c.player.Direction = direction
				}

			default:
				log.Println("unknown message type:", messageType)
			}
		}
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)
			n := len(c.send)
			for i := 0; i < n; i++ {
				w.Write(newline)
				w.Write(<-c.send)
			}

			if err := w.Close(); err != nil {
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func serveWs(hub *Hub, game *Game, w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}

	client := &Client{
		hub:    hub,
		game:   game,
		conn:   conn,
		send:   make(chan []byte, 256),
		player: nil, // Player will be created when client sends join message
	}

	client.hub.register <- client

	go client.writePump()
	go client.readPump()
}
