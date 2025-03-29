// Copyright 2013 The Gorilla WebSocket Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

const (
	// 寫入消息的超時時間
	writeWait = 10 * time.Second

	// 讀取 pong 消息的超時時間
	pongWait = 60 * time.Second

	// 發送 ping 消息的週期，必須小於 pongWait
	pingPeriod = (pongWait * 9) / 10

	// 允許從客戶端接收的最大消息大小
	maxMessageSize = 512
)

var (
	newline = []byte{'\n'}
	space   = []byte{' '}
)

// WebSocket 升級器配置
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // 在開發環境中允許所有來源的連接
	},
}

// Client 是 WebSocket 連接和 Hub 之間的中間層
type Client struct {
	hub *Hub

	// WebSocket 連接
	conn *websocket.Conn

	// 發送消息的緩衝通道
	send chan []byte
}

// readPump 將消息從 WebSocket 連接泵送到 Hub
//
// 應用程序在每個連接的 goroutine 中運行 readPump
// 應用程序通過在該 goroutine 中執行所有讀取操作來確保每個連接最多只有一個讀取器
func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	// 設置讀取限制和超時
	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))

	// 設置 pong 處理器，用於更新讀取超時時間
	c.conn.SetPongHandler(func(string) error { c.conn.SetReadDeadline(time.Now().Add(pongWait)); return nil })

	for {
		// 讀取消息
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("error: %v", err)
			}
			break
		}
		// 處理消息：移除換行符，替換為空格
		message = bytes.TrimSpace(bytes.Replace(message, newline, space, -1))
		// 將消息廣播到 Hub
		c.hub.broadcast <- message
	}
}

// writePump 將消息從 Hub 泵送到 WebSocket 連接
//
// 為每個連接啟動一個運行 writePump 的 goroutine
// 應用程序通過在該 goroutine 中執行所有寫入操作來確保每個連接最多只有一個寫入器
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			// 設置寫入超時
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// Hub 關閉了通道
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			// 獲取寫入器
			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			// 寫入消息
			w.Write(message)

			// 將隊列中的聊天消息添加到當前 WebSocket 消息中
			n := len(c.send)
			for i := 0; i < n; i++ {
				w.Write(newline)
				w.Write(<-c.send)
			}

			// 關閉寫入器
			if err := w.Close(); err != nil {
				return
			}
		case <-ticker.C:
			// 發送 ping 消息
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// serveWs 處理來自客戶端的 WebSocket 請求
func serveWs(hub *Hub, w http.ResponseWriter, r *http.Request) {
	// 升級 HTTP 連接為 WebSocket 連接

	// 使用 := 運算符同時聲明變量並賦值（短變量聲明）
	// := 用於首次聲明變量並賦值，不需要事先聲明變量類型
	// = 用於給已經聲明過的變量賦值，不能用於首次聲明
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}

	// 創建新的客戶端
	client := &Client{hub: hub, conn: conn, send: make(chan []byte, 256)}
	// 註冊客戶端到 Hub
	// 將客戶端註冊到 Hub
	// 這裡使用 channel 操作符 <- 將 client 發送到 hub 的 register 通道
	// 這樣 hub 的 run() 方法就能接收到這個客戶端並將其添加到活動客戶端列表中
	client.hub.register <- client

	// 在後台運行讀取和寫入泵
	go client.writePump()
	go client.readPump()
}
