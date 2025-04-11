package main

import (
	"encoding/json"
	"log"
	"math"
	"sync"
	"time"
)

const (
	// Cooldown duration between hits from the same weapon (in milliseconds)
	weaponHitCooldown = 1000 * time.Millisecond
)

type Game struct {
	Players       []*Player
	mu            sync.Mutex   // Mutex to protect concurrent access to Players
	usedPlayerIDs map[int]bool // Track used player IDs
}

type Player struct {
	ID                  int
	X, Y                float64 // 玩家中心
	Width               float64
	Height              float64
	Health              int
	Direction           float64
	Speed               float64
	WeaponRotationAngle float64 // 武器旋轉的基準角度
	// 玩家擁有的武器列表
	Weapons             []*Weapon
	WeaponRotationSpeed float64

	// Track last hit time for each weapon to implement cooldown
	lastHitByWeapon map[int]time.Time // Map of weaponID -> last hit time
}

type Weapon struct {
	ID            int     // Unique weapon ID to track cooldown
	OwnerID       int     // 所屬玩家ID
	X, Y          float64 // 武器目前的中心
	Width         float64
	Height        float64
	RotationAngle float64 // 武器旋轉的角度
	Damage        int     // 每次碰撞造成的傷害
}

func RectCollision(x1, y1, w1, h1, x2, y2, w2, h2 float64) bool {
	// Adjust coordinates to top-left corner
	x1 -= w1 / 2
	y1 -= h1 / 2
	x2 -= w2 / 2
	y2 -= h2 / 2

	// Simple rectangle collision detection (ignoring rotation)
	if x1 > x2+w2 || x1+w1 < x2 || y1 > y2+h2 || y1+h1 < y2 {
		return false
	}
	return true
}

// // 考慮旋轉的碰撞檢測
// func RotatedRectCollision(x1, y1, w1, h1, angle1, x2, y2, w2, h2, angle2 float64) bool {
// 	// 如果兩個矩形都沒有旋轉，使用簡單的矩形碰撞檢測
// 	if angle1 == 0 && angle2 == 0 {
// 		return RectCollision(x1, y1, w1, h1, x2, y2, w2, h2)
// 	}

// 	// 計算第一個矩形的四個角點
// 	corners1 := calculateCorners(x1, y1, w1, h1, angle1)

// 	// 計算第二個矩形的四個角點
// 	corners2 := calculateCorners(x2, y2, w2, h2, angle2)

// 	// 使用分離軸定理(SAT)檢測兩個旋轉矩形是否碰撞
// 	return checkSATCollision(corners1, corners2)
// }

// // 計算旋轉後矩形的四個角點
// func calculateCorners(x, y, width, height, direction float64) [][2]float64 {
// 	// 矩形中心點
// 	centerX := x + width/2
// 	centerY := y + height/2

// 	// 未旋轉時的四個角點（相對於中心點）
// 	halfW := width / 2
// 	halfH := height / 2
// 	corners := [][2]float64{
// 		{-halfW, -halfH}, // 左上
// 		{halfW, -halfH},  // 右上
// 		{halfW, halfH},   // 右下
// 		{-halfW, halfH},  // 左下
// 	}

// 	// 旋轉並平移每個角點
// 	rotatedCorners := make([][2]float64, 4)
// 	cos := math.Cos(direction)
// 	sin := math.Sin(direction)

// 	for i, corner := range corners {
// 		// 旋轉
// 		rotatedX := corner[0]*cos - corner[1]*sin
// 		rotatedY := corner[0]*sin + corner[1]*cos

// 		// 平移回絕對座標
// 		rotatedCorners[i] = [2]float64{centerX + rotatedX, centerY + rotatedY}
// 	}

// 	return rotatedCorners
// }

// // 使用分離軸定理檢測碰撞
// func checkSATCollision(corners1, corners2 [][2]float64) bool {
// 	// 檢查第一個矩形的邊作為投影軸
// 	for i := 0; i < 4; i++ {
// 		j := (i + 1) % 4
// 		axisX := corners1[j][0] - corners1[i][0]
// 		axisY := corners1[j][1] - corners1[i][1]

// 		// 法向量（垂直於邊的向量）
// 		normalX := -axisY
// 		normalY := axisX

// 		// 如果在這個軸上有間隙，則沒有碰撞
// 		if hasGap(corners1, corners2, normalX, normalY) {
// 			return false
// 		}
// 	}

// 	// 檢查第二個矩形的邊作為投影軸
// 	for i := 0; i < 4; i++ {
// 		j := (i + 1) % 4
// 		axisX := corners2[j][0] - corners2[i][0]
// 		axisY := corners2[j][1] - corners2[i][1]

// 		// 法向量
// 		normalX := -axisY
// 		normalY := axisX

// 		// 如果在這個軸上有間隙，則沒有碰撞
// 		if hasGap(corners1, corners2, normalX, normalY) {
// 			return false
// 		}
// 	}

// 	// 所有軸都沒有間隙，表示有碰撞
// 	return true
// }

// // 檢查在給定軸上是否有間隙
// func hasGap(corners1, corners2 [][2]float64, axisX, axisY float64) bool {
// 	// 標準化軸向量
// 	length := math.Sqrt(axisX*axisX + axisY*axisY)
// 	if length > 0 {
// 		axisX /= length
// 		axisY /= length
// 	}

// 	// 計算第一個矩形在軸上的投影
// 	min1, max1 := projectCorners(corners1, axisX, axisY)

// 	// 計算第二個矩形在軸上的投影
// 	min2, max2 := projectCorners(corners2, axisX, axisY)

// 	// 檢查投影是否有間隙
// 	return max1 < min2 || max2 < min1
// }

// // 將角點投影到軸上並返回最小和最大值
// func projectCorners(corners [][2]float64, axisX, axisY float64) (float64, float64) {
// 	min := math.Inf(1)
// 	max := math.Inf(-1)

// 	for _, corner := range corners {
// 		// 點積計算投影值
// 		projection := corner[0]*axisX + corner[1]*axisY

// 		if projection < min {
// 			min = projection
// 		}
// 		if projection > max {
// 			max = projection
// 		}
// 	}

// 	return min, max
// }

func updateWeaponPosition(p *Player, w *Weapon, direction float64, radius float64) {
	// Only update weapon position if it belongs to the player
	if w.OwnerID == p.ID {
		w.X = p.X + math.Cos(direction)*radius
		w.Y = p.Y + math.Sin(direction)*radius
	}
}

func updatePlayerPosition(p *Player, direction float64, speed float64) {
	p.X += math.Cos(direction) * speed
	p.Y += math.Sin(direction) * speed
}

// 假設 players 是所有玩家的集合
func (g *Game) checkCollisions(hub *Hub) {
	now := time.Now()

	for _, p := range g.Players {
		// 遍歷該玩家所有武器
		for _, w := range p.Weapons {
			// 檢查此武器是否碰撞到其他玩家
			for _, other := range g.Players {
				// 排除自己的武器碰撞自己
				if other.ID == w.OwnerID {
					continue
				}

				// Check for collision
				if RectCollision(w.X, w.Y, w.Width, w.Height, other.X, other.Y, other.Width, other.Height) {
					// Generate a weapon ID that combines owner and weapon index to uniquely identify weapons
					weaponID := w.ID

					// Check cooldown - only apply damage if enough time has passed since last hit
					lastHitTime, hit := other.lastHitByWeapon[weaponID]
					if !hit || now.Sub(lastHitTime) >= weaponHitCooldown {
						// Update the last hit time
						other.lastHitByWeapon[weaponID] = now

						// Apply damage
						other.Health -= w.Damage

						// log.Printf("Player %d 被 Player %d 的武器擊中，扣除 %d 點血，剩餘血量：%d\n",
						// 	other.ID, w.OwnerID, w.Damage, other.Health)

						// Create hit notification
						hitNotification, err := json.Marshal(map[string]interface{}{
							"type":            "playerHit",
							"from":            w.OwnerID,
							"to":              other.ID,
							"damage":          w.Damage,
							"remainingHealth": other.Health,
						})
						if err != nil {
							log.Println("error marshalling player hit info", err)
							continue
						}

						// Broadcast the hit notification to all clients
						// Both the attacker and victim will receive this message
						hub.broadcast <- hitNotification

						// log.Printf("Hit notification sent: Player %d hit Player %d for %d damage",
						// 	w.OwnerID, other.ID, w.Damage)
					}
				}
			}
		}
	}
}

// Release player ID when removed
// If skipLock is true, assumes the mutex is already locked
func (g *Game) removePlayer(playerID int, skipLock bool) {
	if !skipLock {
		g.mu.Lock()
		defer g.mu.Unlock()
	}

	// Remove the player ID from used IDs
	delete(g.usedPlayerIDs, playerID)

	for i, player := range g.Players {
		if player.ID == playerID {
			// Remove player by swapping with the last element and truncating
			lastIndex := len(g.Players) - 1
			g.Players[i] = g.Players[lastIndex]
			g.Players = g.Players[:lastIndex]
			log.Printf("Player %d removed from game", playerID)
			return
		}
	}
}

func (g *Game) removeDeadPlayers(hub *Hub) {
	// Don't acquire the mutex here as it's already locked in the game loop
	removePlayers := []*Player{}
	for _, p := range g.Players {
		if p.Health <= 0 {
			removePlayers = append(removePlayers, p)
		}
	}

	if len(removePlayers) > 0 {
		for _, p := range removePlayers {
			// Send death notification before removing the player
			deathNotification, err := json.Marshal(map[string]interface{}{
				"type":     "playerDeath",
				"playerID": p.ID,
			})
			if err == nil {
				hub.broadcast <- deathNotification
			}

			// Remove the player (skip locking as mutex is already locked)
			g.removePlayer(p.ID, true)
		}
	}
}

func newGame() *Game {
	return &Game{
		Players:       []*Player{},
		usedPlayerIDs: make(map[int]bool),
	}
}

// Generate a unique player ID that hasn't been used before
func (g *Game) generateUniquePlayerID() int {
	g.mu.Lock()
	defer g.mu.Unlock()

	// Try to find an unused ID
	var id int
	for {
		// Generate a random ID between 1 and 1,000,000,000
		id = int(time.Now().UnixNano() % 1000000000)
		if !g.usedPlayerIDs[id] {
			g.usedPlayerIDs[id] = true
			return id
		}
		// In the unlikely case of a collision, we'll generate a new ID
		time.Sleep(time.Nanosecond)
	}
}

// Add a new player with a unique ID to the game and return it
func (g *Game) addNewPlayer() *Player {
	id := g.generateUniquePlayerID()

	player := &Player{
		ID:                  id,
		X:                   100,
		Y:                   100,
		Width:               10,
		Height:              20,
		Health:              100,
		Direction:           0,
		Speed:               100,
		WeaponRotationAngle: 0,
		WeaponRotationSpeed: 1,
		lastHitByWeapon:     make(map[int]time.Time), // Initialize the last hit map
	}

	// Generate two weapons
	weapons := []*Weapon{}
	for range 2 {
		weapons = append(weapons, newWeapon(player))
	}
	player.Weapons = weapons

	// Add the player to the game
	g.mu.Lock()
	g.Players = append(g.Players, player)
	g.mu.Unlock()

	log.Printf("New player %d added to game", id)
	return player
}

func newWeapon(owner *Player) *Weapon {

	// Create a unique weapon ID based on owner ID and weapon index

	index := len(owner.Weapons)

	weaponID := owner.ID*100 + index

	return &Weapon{
		ID:      weaponID,
		OwnerID: owner.ID,
		X:       owner.X,
		Y:       owner.Y,
		Width:   10,
		Height:  20,
		Damage:  40,
	}
}

func (g *Game) run(fps int, hub *Hub) {
	deltaTime := 1.0 / float64(fps)
	ticker := time.NewTicker(time.Second / time.Duration(fps))
	defer ticker.Stop()

	for range ticker.C {
		g.mu.Lock()
		log.Println("Current players:", len(g.Players))
		// Update all players
		for _, p := range g.Players {
			updatePlayerPosition(p, p.Direction, p.Speed*deltaTime)

			// 更新武器旋轉角度
			p.WeaponRotationAngle += p.WeaponRotationSpeed * deltaTime

			weaponCount := len(p.Weapons)
			if weaponCount > 0 {
				// 計算每個武器之間的角度差
				angleDiff := 2 * math.Pi / float64(weaponCount)

				// 為每個武器分配一個均勻分布的角度
				for i, w := range p.Weapons {
					// 計算武器的角度 (基準角度 + 武器的相對角度)
					weaponAngle := p.WeaponRotationAngle + float64(i)*angleDiff
					// 更新武器位置，使其圍繞玩家
					updateWeaponPosition(p, w, weaponAngle, 30)
				}
			}
		}
		// Check for collisions between players
		g.checkCollisions(hub)
		g.removeDeadPlayers(hub)

		playersCopy := make([]*Player, len(g.Players))
		copy(playersCopy, g.Players)
		g.mu.Unlock()

		// Send game state to clients
		jsonData, err := json.Marshal(map[string]interface{}{
			"type":    "gameState",
			"players": playersCopy,
		})
		if err != nil {
			log.Println("error marshalling game info", err)
			continue
		}
		hub.broadcast <- jsonData
	}
}
