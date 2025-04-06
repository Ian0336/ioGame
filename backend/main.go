// Copyright 2013 The Gorilla WebSocket Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net/http"
)

// 設定服務器監聽的地址，預設為 ":8080"
var addr = flag.String("addr", ":8080", "http service address")

// serveHome 處理首頁請求
// 如果訪問的不是根路徑 "/"，返回 404 錯誤
// 如果請求方法不是 GET，返回 405 錯誤
// 否則返回 home.html 頁面
func serveHome(w http.ResponseWriter, r *http.Request) {
	log.Println(r.URL)
	if r.URL.Path != "/" {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	http.ServeFile(w, r, "home.html")
}

func main() {
	// 解析命令行參數
	flag.Parse()

	// 創建一個新的 Hub（聊天室中心）
	hub := newHub()
	// 在後台運行 Hub
	go hub.run()

	game := newGame()
	go game.run(60, hub)

	// 註冊路由處理函數
	http.HandleFunc("/", serveHome) // 處理首頁請求
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		serveWs(hub, w, r) // 處理 WebSocket 連接請求
		player := newPlayer(rand.Intn(100000000))
		game.addPlayer(player)
		log.Println("new player", player.ID)
	})

	fmt.Println("Server is running on port", *addr)
	// 啟動 HTTP 服務器
	err := http.ListenAndServe(*addr, nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
