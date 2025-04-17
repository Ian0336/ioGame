package main

import (
	"encoding/json"
	"log"
	"math"
	"math/rand"
	"sync"
	"time"
)

const (
	// Cooldown duration between hits from the same weapon (in milliseconds)
	weaponHitCooldown = 1000 * time.Millisecond

	// Game boundary constants
	gameMinX = 0
	gameMinY = 0
	gameMaxX = 1200
	gameMaxY = 800
)

// Monster represents a neutral enemy in the game world
// that can be attacked by players and drops a healing potion upon death.
type Monster struct {
	ID        int
	X, Y      float64
	Width     float64
	Height    float64
	Health    int
	MaxHealth int
	Speed     float64
	Direction float64
}

// HealingPotion represents a health recovery item dropped by monsters.
type HealingPotion struct {
	ID     int
	X, Y   float64
	Width  float64
	Height float64
	Amount int // Amount of health restored
}

type Game struct {
	Players        []*Player
	Monsters       []*Monster
	HealingPotions []*HealingPotion
	mu             sync.Mutex   // Mutex to protect concurrent access to Players
	usedPlayerIDs  map[int]bool // Track used player IDs
	usedMonsterIDs map[int]bool // Track used monster IDs
	usedPotionIDs  map[int]bool // Track used potion IDs
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
	Client          *Client
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

func updateWeaponPosition(p *Player, w *Weapon, direction float64, radius float64) {
	// Only update weapon position if it belongs to the player
	if w.OwnerID == p.ID {
		w.X = p.X + math.Cos(direction)*radius
		w.Y = p.Y + math.Sin(direction)*radius
	}
}

func updatePlayerPosition(p *Player, direction float64, speed float64) {
	// Calculate new position
	newX := p.X + math.Cos(direction)*speed
	newY := p.Y + math.Sin(direction)*speed

	// Apply boundary constraints
	// Calculate half width/height for player boundaries
	halfWidth := p.Width / 2
	halfHeight := p.Height / 2

	// Constrain X position
	if newX-halfWidth < gameMinX {
		newX = gameMinX + halfWidth
	} else if newX+halfWidth > gameMaxX {
		newX = gameMaxX - halfWidth
	}

	// Constrain Y position
	if newY-halfHeight < gameMinY {
		newY = gameMinY + halfHeight
	} else if newY+halfHeight > gameMaxY {
		newY = gameMaxY - halfHeight
	}

	// Update player position
	p.X = newX
	p.Y = newY
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

						// Send hit notifications only if clients are not nil
						if p.Client != nil {
							p.Client.send <- hitNotification
						}
						if other.Client != nil {
							other.Client.send <- hitNotification
						}

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
			if player.Client != nil {
				player.Client.player = nil
			}
			return
		}
	}
}

func (g *Game) removeDeadPlayers() {
	// Don't acquire the mutex here as it's already locked in the game loop
	removePlayers := []*Player{}
	for _, p := range g.Players {
		if p.Health <= 0 {
			removePlayers = append(removePlayers, p)
		}
	}

	if len(removePlayers) > 0 {
		for _, p := range removePlayers {
			// Only send death notification if Client is not nil
			if p.Client != nil {
				deathNotification, err := json.Marshal(map[string]interface{}{
					"type":     "playerDeath",
					"playerID": p.ID,
				})
				if err == nil {
					p.Client.send <- deathNotification
				}
			}

			// Remove the player (skip locking as mutex is already locked)
			g.removePlayer(p.ID, true)
		}
	}
}

func newGame() *Game {
	return &Game{
		Players:        []*Player{},
		Monsters:       []*Monster{},
		HealingPotions: []*HealingPotion{},
		usedPlayerIDs:  make(map[int]bool),
		usedMonsterIDs: make(map[int]bool),
		usedPotionIDs:  make(map[int]bool),
	}
}

// Generate a unique player ID that hasn't been used before
func (g *Game) generateUniquePlayerID() int {
	var id int
	for {
		id = int(time.Now().UnixNano() % 1000000000)
		if !g.usedPlayerIDs[id] {
			return id
		}
		time.Sleep(time.Nanosecond)
	}
}

// Add a new player with a unique ID to the game and return it
func (g *Game) addNewPlayer(client *Client) *Player {
	g.mu.Lock()
	defer g.mu.Unlock()

	id := g.generateUniquePlayerID()
	g.usedPlayerIDs[id] = true

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
		Client:              client,
	}

	// Generate two weapons
	weapons := []*Weapon{}
	for range 2 {
		weapons = append(weapons, newWeapon(player))
	}
	player.Weapons = weapons

	g.Players = append(g.Players, player)

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

// Generate a unique monster ID that hasn't been used before
func (g *Game) generateUniqueMonsterID() int {
	var id int
	for {
		id = int(time.Now().UnixNano() % 1000000000)
		if !g.usedMonsterIDs[id] {
			return id
		}
		time.Sleep(time.Nanosecond)
	}
}

// Spawn a monster at a random location
// If skipLock is true, assumes the mutex is already locked
func (g *Game) spawnMonster(skipLock bool) *Monster {
	if !skipLock {
		g.mu.Lock()
		defer g.mu.Unlock()
	}
	id := g.generateUniqueMonsterID()
	g.usedMonsterIDs[id] = true
	monster := &Monster{
		ID:        id,
		X:         gameMinX + 50 + rand.Float64()*(gameMaxX-gameMinX-100),
		Y:         gameMinY + 50 + rand.Float64()*(gameMaxY-gameMinY-100),
		Width:     20,
		Height:    20,
		Health:    60,
		MaxHealth: 60,
		Speed:     30,
	}
	g.Monsters = append(g.Monsters, monster)
	return monster
}

// Generate a unique potion ID that hasn't been used before
func (g *Game) generateUniquePotionID() int {
	var id int
	for {
		id = int(time.Now().UnixNano() % 1000000000)
		if !g.usedPotionIDs[id] {
			return id
		}
		time.Sleep(time.Nanosecond)
	}
}

// Spawn a healing potion at the given location
// If skipLock is true, assumes the mutex is already locked
func (g *Game) spawnHealingPotion(x, y float64, skipLock bool) *HealingPotion {
	if !skipLock {
		g.mu.Lock()
		defer g.mu.Unlock()
	}
	id := g.generateUniquePotionID()
	g.usedPotionIDs[id] = true
	potion := &HealingPotion{
		ID:     id,
		X:      x,
		Y:      y,
		Width:  12,
		Height: 12,
		Amount: 40,
	}
	g.HealingPotions = append(g.HealingPotions, potion)
	return potion
}

// Remove dead monsters and drop healing potions
// If skipLock is true, assumes the mutex is already locked
func (g *Game) removeDeadMonstersAndDropPotions(skipLock bool) {
	if !skipLock {
		g.mu.Lock()
		defer g.mu.Unlock()
	}
	remaining := []*Monster{}
	for _, m := range g.Monsters {
		if m.Health <= 0 {
			// Drop a healing potion at monster's position
			g.spawnHealingPotion(m.X, m.Y, true) // Skip lock since we're already locked
			delete(g.usedMonsterIDs, m.ID)
			continue
		}
		remaining = append(remaining, m)
	}
	g.Monsters = remaining
}

// Update monster logic (simple AI: move randomly or stay still for now)
func (g *Game) updateMonsters(deltaTime float64) {
	// For now, monsters do not move. You can add movement logic here.
	for _, m := range g.Monsters {
		m.X += rand.Float64() * 10 * deltaTime
		m.Y += rand.Float64() * 10 * deltaTime

	}

}

// Check for collisions between player weapons and monsters
func (g *Game) checkMonsterCollisions(hub *Hub) {
	for _, p := range g.Players {
		for _, w := range p.Weapons {
			for _, m := range g.Monsters {
				if m.Health > 0 && RectCollision(w.X, w.Y, w.Width, w.Height, m.X, m.Y, m.Width, m.Height) {
					// Apply damage
					m.Health -= w.Damage

					// Only notify client if it exists
					if p.Client != nil {
						// Determine if monster was killed by this hit
						monsterKilled := m.Health <= 0
						status := "hit"
						if monsterKilled {
							status = "killed"
						}

						// Create monster hit notification
						hitNotification, err := json.Marshal(map[string]interface{}{
							"type":          "monsterHit",
							"playerID":      p.ID,
							"monsterID":     m.ID,
							"damage":        w.Damage,
							"monsterHealth": m.Health,
							"status":        status,
						})
						if err == nil {
							p.Client.send <- hitNotification
						}
					}
				}
			}
		}
	}
}

// Check for collisions between players and healing potions
func (g *Game) checkPotionCollisions(hub *Hub) {
	remainingPotions := []*HealingPotion{}
	for _, potion := range g.HealingPotions {
		collected := false
		for _, p := range g.Players {
			if RectCollision(p.X, p.Y, p.Width, p.Height, potion.X, potion.Y, potion.Width, potion.Height) {
				// Heal the player
				oldHealth := p.Health
				p.Health += potion.Amount
				if p.Health > 100 {
					p.Health = 100
				}

				// Notify player about potion collection
				if p.Client != nil {
					potionNotification, err := json.Marshal(map[string]interface{}{
						"type":         "potionCollected",
						"playerID":     p.ID,
						"potionID":     potion.ID,
						"amount":       potion.Amount,
						"healedAmount": p.Health - oldHealth,
						"newHealth":    p.Health,
					})
					if err == nil {
						p.Client.send <- potionNotification
					}
				}

				delete(g.usedPotionIDs, potion.ID)
				collected = true
				break
			}
		}
		if !collected {
			remainingPotions = append(remainingPotions, potion)
		}
	}
	g.HealingPotions = remainingPotions
}

// Update the game loop to handle monsters and potions, passing skipLock=true
func (g *Game) run(fps int, hub *Hub) {
	deltaTime := 1.0 / float64(fps)
	ticker := time.NewTicker(time.Second / time.Duration(fps))
	defer ticker.Stop()

	for range ticker.C {
		g.mu.Lock()
		// Update all players
		for _, p := range g.Players {
			updatePlayerPosition(p, p.Direction, p.Speed*deltaTime)
			p.WeaponRotationAngle += p.WeaponRotationSpeed * deltaTime
			weaponCount := len(p.Weapons)
			if weaponCount > 0 {
				angleDiff := 2 * math.Pi / float64(weaponCount)
				for i, w := range p.Weapons {
					weaponAngle := p.WeaponRotationAngle + float64(i)*angleDiff
					updateWeaponPosition(p, w, weaponAngle, 30)
				}
			}
		}

		// --- Monster logic ---
		// Spawn monsters if needed (1 monster per player)
		if len(g.Monsters) < len(g.Players) {
			for i := len(g.Monsters); i < len(g.Players); i++ {
				g.spawnMonster(true) // Skip lock since we're already locked
			}
		}
		g.updateMonsters(deltaTime)
		g.checkMonsterCollisions(hub)
		g.removeDeadMonstersAndDropPotions(true) // Skip lock since we're already locked

		// --- Potion logic ---
		g.checkPotionCollisions(hub)

		// --- Player logic ---
		g.checkCollisions(hub)
		g.removeDeadPlayers()

		playersCopy := make([]*Player, len(g.Players))
		copy(playersCopy, g.Players)
		monstersCopy := make([]*Monster, len(g.Monsters))
		copy(monstersCopy, g.Monsters)
		potionsCopy := make([]*HealingPotion, len(g.HealingPotions))
		copy(potionsCopy, g.HealingPotions)
		g.mu.Unlock()

		// Send game state to clients (now includes monsters and potions)
		jsonData, err := json.Marshal(map[string]interface{}{
			"type":     "gameState",
			"players":  playersCopy,
			"monsters": monstersCopy,
			"potions":  potionsCopy,
		})
		if err != nil {
			log.Println("error marshalling game info", err)
			continue
		}
		hub.broadcast <- jsonData
	}
}
