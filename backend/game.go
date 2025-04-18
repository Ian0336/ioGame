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
	playerMasterRatio = 2
	baseMonsterAmount = 10
	maxMonsterAmount  = 60

	// Game boundary constants
	gameMinX = 0
	gameMinY = 0
	gameMaxX = 1200
	gameMaxY = 800
)

// Entity is the base struct for all game objects with position and size
type Entity struct {
	ID     int
	X, Y   float64
	Width  float64
	Height float64
}

// Movable is an interface for entities that can move
type Movable interface {
	Move(deltaTime float64)
}

// Collidable is an interface for entities that can be checked for collision
type Collidable interface {
	CheckCollision(other *Entity) bool
	GetEntity() *Entity
}

// Hittable is an interface for entities that can take damage
type Hittable interface {
	Collidable
	TakeDamage(damage int)
	IsDead() bool
}

// ItemCollector is an interface for entities that can collect items
type ItemCollector interface {
	Collidable
	CollectItem(item *Item)
}

// Item is an interface for collectible entities
type Item interface {
	Collidable
	OnCollect(collector ItemCollector)
}

// GetEntity returns the base Entity of an object
func (e *Entity) GetEntity() *Entity {
	return e
}

// RectCollision checks for collision between two entities
func (e *Entity) CheckCollision(other *Entity) bool {
	// Adjust coordinates to top-left corner
	x1 := e.X - e.Width/2
	y1 := e.Y - e.Height/2
	x2 := other.X - other.Width/2
	y2 := other.Y - other.Height/2

	// Simple rectangle collision detection (ignoring rotation)
	if x1 > x2+other.Width || x1+e.Width < x2 || y1 > y2+other.Height || y1+e.Height < y2 {
		return false
	}
	return true
}

// Health component for entities with health
type HealthComponent struct {
	Health      int
	MaxHealth   int
	lastHitById map[int]time.Time
}

// TakeDamage reduces health by the given amount
func (h *HealthComponent) TakeDamage(damage int) {
	h.Health -= damage
	if h.Health < 0 {
		h.Health = 0
	}
}

// Heal increases health by the given amount
// returns the real healed amount
func (h *HealthComponent) Heal(amount int) int {
	oldHealth := h.Health
	h.Health += amount
	if h.Health > h.MaxHealth {
		h.Health = h.MaxHealth
		return h.MaxHealth - oldHealth
	}
	return amount
}

// IsDead returns true if health is 0 or less
func (h *HealthComponent) IsDead() bool {
	return h.Health <= 0
}

// Check hit cooldown
func (h *HealthComponent) CheckHitCooldown(weaponID int) bool {
	lastHitTime, hit := h.lastHitById[weaponID]
	if !hit || time.Since(lastHitTime) >= weaponHitCooldown {
		h.lastHitById[weaponID] = time.Now()
		return true
	}
	return false
}

// ExperienceComponent for entities that can gain experience
type ExperienceComponent struct {
	Experience int
	Level      int
}

// Movement component for entities that move
type MovementComponent struct {
	Speed     float64
	Direction float64
}

// AttackComponent for entities that can attack
type AttackComponent struct {
	Damage int
}

// Player represents a user-controlled character
type Player struct {
	Entity
	HealthComponent
	MovementComponent
	ExperienceComponent
	AttackComponent
	WeaponRotationAngle float64
	WeaponRotationSpeed float64
	Weapons             []*Weapon
	Client              *Client
}

// Move updates the player's position based on direction and speed
func (p *Player) Move(deltaTime float64) {
	// Calculate new position
	newX := p.X + math.Cos(p.Direction)*p.Speed*deltaTime
	newY := p.Y + math.Sin(p.Direction)*p.Speed*deltaTime

	// Apply boundary constraints
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

// CollectItem handles item collection for the player
func (p *Player) CollectItem(item *Item) {
	// Implemented by specific item types
}

// Weapon represents a player's weapon
type Weapon struct {
	Entity
	OwnerID       int
	RotationAngle float64
}

// Monster represents an enemy in the game
type Monster struct {
	Entity
	HealthComponent
	MovementComponent
	AttackComponent
	DropRate float64
}

// Move updates the monster's position
func (m *Monster) Move(deltaTime float64) {
	// Simple random movement
	newX := m.X + math.Cos(m.Direction)*m.Speed*deltaTime
	newY := m.Y + math.Sin(m.Direction)*m.Speed*deltaTime

	// Apply boundary constraints
	halfWidth := m.Width / 2
	halfHeight := m.Height / 2

	// Constrain X position
	if newX-halfWidth < gameMinX {
		newX = gameMinX + halfWidth
		m.Direction = math.Pi - m.Direction // Bounce
	} else if newX+halfWidth > gameMaxX {
		newX = gameMaxX - halfWidth
		m.Direction = math.Pi - m.Direction // Bounce
	}

	// Constrain Y position
	if newY-halfHeight < gameMinY {
		newY = gameMinY + halfHeight
		m.Direction = -m.Direction // Bounce
	} else if newY+halfHeight > gameMaxY {
		newY = gameMaxY - halfHeight
		m.Direction = -m.Direction // Bounce
	}

	// Update monster position
	m.X = newX
	m.Y = newY

	// Occasionally change direction
	if rand.Float64() < 0.01 {
		m.Direction = rand.Float64() * 2 * math.Pi
	}
}

// HealingPotion represents a health recovery item
type HealingPotion struct {
	Entity
	Amount int
}

// OnCollect handles what happens when the potion is collected
func (h *HealingPotion) OnCollect(collector ItemCollector) {
	if player, ok := collector.(*Player); ok {
		if player.Client == nil {
			return
		}
		healedAmount := player.Heal(h.Amount)

		// Notify player about potion collection if possible
		potionNotification, err := json.Marshal(map[string]interface{}{
			"type":         "potionCollected",
			"playerID":     player.ID,
			"potionID":     h.ID,
			"amount":       h.Amount,
			"healedAmount": healedAmount,
			"newHealth":    player.Health,
		})
		if err == nil {
			player.Client.send <- potionNotification
		}
	}
}

// Experience represents experience points that can be collected
type Experience struct {
	Entity
	Amount int
}

// OnCollect handles what happens when experience is collected
func (e *Experience) OnCollect(collector ItemCollector) {
	if player, ok := collector.(*Player); ok {
		if player.Client == nil {
			return
		}
		player.Experience += e.Amount
		if player.Experience >= player.Level*10 {
			player.Level++
			player.Damage++
			player.MaxHealth += 10
			player.Health = player.MaxHealth
			player.Experience = 0
			levelUpNotification, err := json.Marshal(map[string]interface{}{
				"type":     "levelUp",
				"playerID": player.ID,
				"level":    player.Level,
			})
			if err == nil {
				player.Client.send <- levelUpNotification
			}
		}

		// Notify player about experience collection if possible

		expNotification, err := json.Marshal(map[string]interface{}{
			"type":            "experienceCollected",
			"playerID":        player.ID,
			"experienceID":    e.ID,
			"amount":          e.Amount,
			"totalExperience": player.Experience,
		})
		if err == nil {
			player.Client.send <- expNotification
		}
	}
}

// Game represents the game state and systems
type Game struct {
	Players        []*Player
	Monsters       []*Monster
	HealingPotions []*HealingPotion
	Experiences    []*Experience
	mu             sync.Mutex
	usedIDs        map[string]map[int]bool // Tracks used IDs by type (player, monster, potion)
}

// newGame creates a new game instance
func newGame() *Game {
	g := &Game{
		Players:        []*Player{},
		Monsters:       []*Monster{},
		HealingPotions: []*HealingPotion{},
		Experiences:    []*Experience{},
		usedIDs:        make(map[string]map[int]bool),
	}
	g.usedIDs["player"] = make(map[int]bool)
	g.usedIDs["monster"] = make(map[int]bool)
	g.usedIDs["potion"] = make(map[int]bool)
	g.usedIDs["weapon"] = make(map[int]bool)
	g.usedIDs["experience"] = make(map[int]bool)
	return g
}

// generateID generates a unique ID for a given entity type
func (g *Game) generateID(entityType string) int {
	var id int
	for {
		id = int(time.Now().UnixNano() % 1000000000)
		if !g.usedIDs[entityType][id] {
			g.usedIDs[entityType][id] = true
			return id
		}
		time.Sleep(time.Nanosecond)
	}
}

// releaseID releases an ID when an entity is removed
func (g *Game) releaseID(entityType string, id int) {
	delete(g.usedIDs[entityType], id)
}

// addNewPlayer creates and adds a new player to the game
func (g *Game) addNewPlayer(client *Client) *Player {
	g.mu.Lock()
	defer g.mu.Unlock()

	id := g.generateID("player")

	player := &Player{
		Entity: Entity{
			ID:     id,
			X:      100,
			Y:      100,
			Width:  10,
			Height: 20,
		},
		HealthComponent: HealthComponent{
			Health:      100,
			MaxHealth:   100,
			lastHitById: make(map[int]time.Time),
		},
		ExperienceComponent: ExperienceComponent{
			Experience: 0,
			Level:      1,
		},
		MovementComponent: MovementComponent{
			Speed:     100,
			Direction: 0,
		},
		AttackComponent: AttackComponent{
			Damage: 10,
		},
		WeaponRotationAngle: 0,
		WeaponRotationSpeed: 1,
		Client:              client,
		Weapons:             []*Weapon{},
	}

	// Generate two weapons
	for i := 0; i < 2; i++ {
		player.Weapons = append(player.Weapons, g.newWeapon(player))
	}

	g.Players = append(g.Players, player)
	log.Printf("New player %d added to game", id)
	return player
}

// newWeapon creates a new weapon for a player
func (g *Game) newWeapon(owner *Player) *Weapon {
	weaponID := g.generateID("weapon")

	return &Weapon{
		Entity: Entity{
			ID:     weaponID,
			X:      owner.X,
			Y:      owner.Y,
			Width:  10,
			Height: 20,
		},
		OwnerID: owner.ID,
	}
}

// spawnMonster creates and adds a new monster to the game
func (g *Game) spawnMonster(skipLock bool) *Monster {
	if !skipLock {
		g.mu.Lock()
		defer g.mu.Unlock()
	}

	id := g.generateID("monster")
	monster := &Monster{
		Entity: Entity{
			ID:     id,
			X:      gameMinX + 50 + rand.Float64()*(gameMaxX-gameMinX-100),
			Y:      gameMinY + 50 + rand.Float64()*(gameMaxY-gameMinY-100),
			Width:  20,
			Height: 20,
		},
		HealthComponent: HealthComponent{
			Health:      60,
			MaxHealth:   60,
			lastHitById: make(map[int]time.Time),
		},
		MovementComponent: MovementComponent{
			Speed:     30,
			Direction: rand.Float64() * 2 * math.Pi,
		},
		AttackComponent: AttackComponent{
			Damage: 20,
		},
		DropRate: 0.75,
	}
	g.Monsters = append(g.Monsters, monster)
	return monster
}

// spawnHealingPotion creates and adds a new healing potion to the game
func (g *Game) spawnHealingPotion(x, y float64, skipLock bool) *HealingPotion {
	if !skipLock {
		g.mu.Lock()
		defer g.mu.Unlock()
	}

	id := g.generateID("potion")
	potion := &HealingPotion{
		Entity: Entity{
			ID:     id,
			X:      x,
			Y:      y,
			Width:  12,
			Height: 12,
		},
		Amount: 25,
	}
	g.HealingPotions = append(g.HealingPotions, potion)
	return potion
}

// spawnExperience creates and adds experience points to the game
func (g *Game) spawnExperience(x, y float64, amount int, skipLock bool) *Experience {
	if !skipLock {
		g.mu.Lock()
		defer g.mu.Unlock()
	}

	id := g.generateID("experience")

	// Random offset from the origin point (where the entity died)
	offsetX := (rand.Float64() - 0.5) * 30
	offsetY := (rand.Float64() - 0.5) * 30

	exp := &Experience{
		Entity: Entity{
			ID:     id,
			X:      x + offsetX,
			Y:      y + offsetY,
			Width:  8,
			Height: 8,
		},
		Amount: amount,
	}
	g.Experiences = append(g.Experiences, exp)
	return exp
}

// removePlayer removes a player from the game
func (g *Game) removePlayer(playerID int, skipLock bool) {
	if !skipLock {
		g.mu.Lock()
		defer g.mu.Unlock()
	}

	g.releaseID("player", playerID)

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

// CollisionSystem handles collisions between different game entities
type CollisionSystem struct {
	game *Game
	hub  *Hub
}

// NewCollisionSystem creates a new collision system
func NewCollisionSystem(game *Game, hub *Hub) *CollisionSystem {
	return &CollisionSystem{game: game, hub: hub}
}

// Update checks and handles all game collisions
func (cs *CollisionSystem) Update() {
	cs.checkWeaponCollisions()
	cs.checkMonsterPlayerCollisions()
	cs.checkPlayerPotionCollisions()
	cs.checkPlayerExperienceCollisions()
}

// checkWeaponCollisions handles weapon-to-player collisions and weapon-to-monster collisions
func (cs *CollisionSystem) checkWeaponCollisions() {

	for _, p := range cs.game.Players {
		for _, w := range p.Weapons {
			for _, other := range cs.game.Players {
				if other.ID == w.OwnerID {
					continue // Skip owner
				}

				if w.CheckCollision(other.GetEntity()) {
					// Check cooldown
					weaponID := w.ID
					if other.CheckHitCooldown(weaponID) {
						other.TakeDamage(p.Damage)

						// Create hit notification
						hitNotification, err := json.Marshal(map[string]interface{}{
							"type":            "playerHit",
							"from":            w.OwnerID,
							"to":              other.ID,
							"damage":          p.Damage,
							"remainingHealth": other.Health,
						})
						if err == nil {
							if p.Client != nil {
								p.Client.send <- hitNotification
							}
							if other.Client != nil {
								other.Client.send <- hitNotification
							}
						}
					}
				}
			}
			for _, m := range cs.game.Monsters {
				if m.Health > 0 && w.CheckCollision(m.GetEntity()) {
					if !m.CheckHitCooldown(w.ID) {
						continue
					}
					// Apply damage
					m.TakeDamage(p.Damage)

					// Only notify client if it exists
					if p.Client != nil {
						// Determine if monster was killed by this hit
						status := "hit"
						if m.IsDead() {
							status = "killed"
						}

						// Create monster hit notification
						hitNotification, err := json.Marshal(map[string]interface{}{
							"type":          "monsterHit",
							"playerID":      p.ID,
							"monsterID":     m.ID,
							"damage":        p.Damage,
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

// checkMonsterPlayerCollisions handles monster-to-player collisions
func (cs *CollisionSystem) checkMonsterPlayerCollisions() {
	for _, m := range cs.game.Monsters {
		for _, p := range cs.game.Players {
			if m.Health > 0 && m.CheckCollision(p.GetEntity()) {
				if !p.CheckHitCooldown(m.ID) {
					continue
				}
				// Apply damage
				p.TakeDamage(m.Damage)

				// Only notify client if it exists
				if p.Client != nil {
					hitNotification, err := json.Marshal(map[string]interface{}{
						"type":            "playerHit",
						"from":            m.ID,
						"to":              p.ID,
						"damage":          m.Damage,
						"remainingHealth": p.Health,
					})
					if err == nil {
						p.Client.send <- hitNotification
					}
				}
			}
		}
	}
}

// checkPlayerPotionCollisions handles player-to-potion collisions
func (cs *CollisionSystem) checkPlayerPotionCollisions() {
	remainingPotions := []*HealingPotion{}
	for _, potion := range cs.game.HealingPotions {
		collected := false
		for _, p := range cs.game.Players {
			if potion.CheckCollision(p.GetEntity()) {
				potion.OnCollect(p)

				cs.game.releaseID("potion", potion.ID)
				collected = true
				break
			}
		}
		if !collected {
			remainingPotions = append(remainingPotions, potion)
		}
	}
	cs.game.HealingPotions = remainingPotions
}

// checkPlayerExperienceCollisions handles player-to-experience collisions
func (cs *CollisionSystem) checkPlayerExperienceCollisions() {
	remainingExperiences := []*Experience{}
	for _, exp := range cs.game.Experiences {
		collected := false
		for _, p := range cs.game.Players {
			if exp.CheckCollision(p.GetEntity()) {
				// Add experience to player
				exp.OnCollect(p)

				cs.game.releaseID("experience", exp.ID)
				collected = true
				break
			}
		}
		if !collected {
			remainingExperiences = append(remainingExperiences, exp)
		}
	}
	cs.game.Experiences = remainingExperiences
}

// MonsterSystem handles monster spawning, updates, and cleanup
type MonsterSystem struct {
	game *Game
}

// NewMonsterSystem creates a new monster system
func NewMonsterSystem(game *Game) *MonsterSystem {
	return &MonsterSystem{game: game}
}

// Update updates all monsters and handles spawning and removal
func (ms *MonsterSystem) Update(deltaTime float64) {
	// Spawn monsters if needed (1 monster per player)
	for len(ms.game.Monsters) < len(ms.game.Players)*playerMasterRatio+baseMonsterAmount && len(ms.game.Monsters) < maxMonsterAmount {
		ms.game.spawnMonster(true)
	}

	// Update monster positions
	for _, m := range ms.game.Monsters {
		m.Move(deltaTime)
	}

	// Remove dead monsters and drop potions
	ms.removeDeadMonsters()
}

// removeDeadMonsters removes dead monsters and drops healing potions
func (ms *MonsterSystem) removeDeadMonsters() {
	remaining := []*Monster{}
	for _, m := range ms.game.Monsters {
		if m.IsDead() {
			// Drop a healing potion at monster's position
			if rand.Float64() < m.DropRate {
				ms.game.spawnHealingPotion(m.X, m.Y, true)
			}

			// Spawn experience points
			expAmount := 10 + rand.Intn(10) // 10-19 experience points
			numExpOrbs := 3 + rand.Intn(3)  // 3-5 experience orbs

			for i := 0; i < numExpOrbs; i++ {
				ms.game.spawnExperience(m.X, m.Y, expAmount/numExpOrbs, true)
			}

			ms.game.releaseID("monster", m.ID)
			continue
		}
		remaining = append(remaining, m)
	}
	ms.game.Monsters = remaining
}

// PlayerSystem handles player updates and cleanup
type PlayerSystem struct {
	game *Game
	hub  *Hub
}

// NewPlayerSystem creates a new player system
func NewPlayerSystem(game *Game, hub *Hub) *PlayerSystem {
	return &PlayerSystem{game: game, hub: hub}
}

// Update updates all players and handles removal of dead players
func (ps *PlayerSystem) Update(deltaTime float64) {
	// Update all players
	for _, p := range ps.game.Players {
		// Update player position
		p.Move(deltaTime)

		// Update player damage and max health with experience
		p.Damage = p.Damage + p.Experience/100
		p.MaxHealth = p.MaxHealth + p.Experience/100
		log.Printf("Player %d has %d damage and %d max health", p.Experience, p.Damage, p.MaxHealth)

		// Update weapon rotation
		p.WeaponRotationAngle += p.WeaponRotationSpeed * deltaTime

		// Update weapon positions
		weaponCount := len(p.Weapons)
		if weaponCount > 0 {
			angleDiff := 2 * math.Pi / float64(weaponCount)
			for i, w := range p.Weapons {
				weaponAngle := p.WeaponRotationAngle + float64(i)*angleDiff
				radius := 30.0
				w.X = p.X + math.Cos(weaponAngle)*radius
				w.Y = p.Y + math.Sin(weaponAngle)*radius
			}
		}
	}

	// Remove dead players
	ps.removeDeadPlayers()
}

// removeDeadPlayers removes players with health <= 0
func (ps *PlayerSystem) removeDeadPlayers() {
	removePlayers := []*Player{}
	for _, p := range ps.game.Players {
		if p.IsDead() {
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

			// Spawn experience when player dies (half of their current experience)
			if p.Experience > 0 {
				expAmount := p.Experience / 2
				numExpOrbs := 4 + rand.Intn(4) // 4-7 experience orbs

				for i := 0; i < numExpOrbs; i++ {
					ps.game.spawnExperience(p.X, p.Y, expAmount/numExpOrbs, true)
				}
			}

			// Remove the player
			ps.game.removePlayer(p.ID, true)
		}
	}
}

// run starts the game loop
func (g *Game) run(fps int, hub *Hub) {
	deltaTime := 1.0 / float64(fps)
	ticker := time.NewTicker(time.Second / time.Duration(fps))
	defer ticker.Stop()

	// Initialize systems
	playerSystem := NewPlayerSystem(g, hub)
	monsterSystem := NewMonsterSystem(g)
	collisionSystem := NewCollisionSystem(g, hub)

	for range ticker.C {
		g.mu.Lock()

		// Update all systems
		playerSystem.Update(deltaTime)
		monsterSystem.Update(deltaTime)
		collisionSystem.Update()

		// Create copies of the game state to send to clients
		playersCopy := make([]*Player, len(g.Players))
		copy(playersCopy, g.Players)
		monstersCopy := make([]*Monster, len(g.Monsters))
		copy(monstersCopy, g.Monsters)
		potionsCopy := make([]*HealingPotion, len(g.HealingPotions))
		copy(potionsCopy, g.HealingPotions)
		experiencesCopy := make([]*Experience, len(g.Experiences))
		copy(experiencesCopy, g.Experiences)

		g.mu.Unlock()

		// Send game state to clients
		jsonData, err := json.Marshal(map[string]interface{}{
			"type":        "gameState",
			"players":     playersCopy,
			"monsters":    monstersCopy,
			"potions":     potionsCopy,
			"experiences": experiencesCopy,
		})
		if err != nil {
			log.Println("error marshalling game info", err)
			continue
		}
		hub.broadcast <- jsonData
	}
}
