// Copyright 2013 The Gorilla WebSocket Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

// Hub 維護活動客戶端集合並向客戶端廣播消息
type Hub struct {
	// 已註冊的客戶端
	clients map[*Client]bool

	// 來自客戶端的入站消息
	broadcast chan []byte

	// 客戶端的註冊請求
	register chan *Client

	// 客戶端的註銷請求
	unregister chan *Client
}

// newHub 創建一個新的 Hub
func newHub() *Hub {
	return &Hub{
		broadcast:  make(chan []byte),      // 創建廣播通道
		register:   make(chan *Client),     // 創建註冊通道
		unregister: make(chan *Client),     // 創建註銷通道
		clients:    make(map[*Client]bool), // 初始化客戶端映射
	}
}

// run 運行 Hub，處理客戶端的註冊、註銷和消息廣播
func (h *Hub) run() {
	for {
		select {
		case client := <-h.register:
			// 註冊新客戶端
			h.clients[client] = true

		case client := <-h.unregister:
			// 註銷客戶端
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}

		case message := <-h.broadcast:
			// 向所有客戶端廣播消息
			for client := range h.clients {
				select {
				case client.send <- message:
					// 成功發送消息
				default:
					// 如果發送失敗，關閉客戶端連接並從 Hub 中移除
					close(client.send)
					delete(h.clients, client)
				}
			}
		}
	}
}
