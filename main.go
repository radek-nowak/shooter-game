package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"image/color"
	"log"
	"math"
	"net"
	"os"
	"sync"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
)

const (
	ScreenWidth             = 800
	ScreenHeight            = 600
	PlayerSpeed             = 1.0
	PlayerSprintSpeedFactor = 1.7
	BulletSpeed             = 6.0
	PlayerRadius            = 10.0
	BulletRadius            = 3.0
	MaxHealth               = 100
	ShootCooldown           = 200 * time.Millisecond
	ServerPort              = ":8080"
)

type EventType string

const (
	EventTypePlayerUpdate EventType = "player_update"
	EventTypePlayerHit    EventType = "player_hit"
)

type Player struct {
	ID       string    `json:"id"`
	X        float64   `json:"x"`
	Y        float64   `json:"y"`
	Angle    float64   `json:"angle"`
	Health   int       `json:"health"`
	Bullets  []*Bullet `json:"bullets"`
	lastShot time.Time `json:"-"`
}

type Bullet struct {
	OwnerID string  `json:"owner_id"`
	X       float64 `json:"x"`
	Y       float64 `json:"y"`
	DX      float64 `json:"dx"`
	DY      float64 `json:"dy"`
}

type Obstacle struct {
	X      float64
	Y      float64
	Width  float64
	Height float64
}

type Event struct {
	Type EventType       `json:"type"`
	Data json.RawMessage `json:"data"`
}

type PlayerUpdate struct {
	ID      string    `json:"id"`
	X       float64   `json:"x"`
	Y       float64   `json:"y"`
	Angle   float64   `json:"angle"`
	Health  int       `json:"health"`
	Bullets []*Bullet `json:"bullets"`
}

type PlayerHit struct {
	VictimID string `json:"victim_id"`
	Damage   int    `json:"damage"`
}

type Game struct {
	player    *Player
	players   map[string]*Player
	obstacles []*Obstacle
	conn      net.Conn
	mu        sync.Mutex
}

func NewPlayer(id string, x, y float64) *Player {
	return &Player{
		ID:      id,
		X:       x,
		Y:       y,
		Health:  MaxHealth,
		Bullets: []*Bullet{},
	}
}

func NewObstacles() []*Obstacle {
	return []*Obstacle{
		{X: 200, Y: 150, Width: 100, Height: 200},
		{X: 500, Y: 300, Width: 150, Height: 100},
	}
}

func (p *Player) Update(obstacles []*Obstacle) {
	if p.Health <= 0 {
		return
	}

	moveX, moveY := 0.0, 0.0

	movementSpeed := PlayerSpeed

	if ebiten.IsKeyPressed(ebiten.KeyShiftLeft) {
		movementSpeed *= PlayerSprintSpeedFactor
	}

	if ebiten.IsKeyPressed(ebiten.KeyW) {
		moveY -= movementSpeed
	}
	if ebiten.IsKeyPressed(ebiten.KeyS) {
		moveY += movementSpeed
	}
	if ebiten.IsKeyPressed(ebiten.KeyA) {
		moveX -= movementSpeed
	}
	if ebiten.IsKeyPressed(ebiten.KeyD) {
		moveX += movementSpeed
	}

	// Move horizontally and check collision
	p.X += moveX
	if collidesWithObstacles(p.X, p.Y, PlayerRadius, obstacles) {
		p.X -= moveX // Revert horizontal movement if collides
	}

	// Move vertically and check collision
	p.Y += moveY
	if collidesWithObstacles(p.X, p.Y, PlayerRadius, obstacles) {
		p.Y -= moveY // Revert vertical movement if collides
	}

	// Update aiming angle
	mx, my := ebiten.CursorPosition()
	dx, dy := float64(mx)-p.X, float64(my)-p.Y
	p.Angle = math.Atan2(dy, dx)

	// Shooting
	if ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) && time.Since(p.lastShot) > ShootCooldown {
		p.Shoot()
		p.lastShot = time.Now()
	}

	// Update bullets
	for i := len(p.Bullets) - 1; i >= 0; i-- {
		p.Bullets[i].Update()
		if p.Bullets[i].OutOfBounds() || bulletHitsObstacle(p.Bullets[i], obstacles) {
			p.Bullets = append(p.Bullets[:i], p.Bullets[i+1:]...)
		}
	}
}

func (p *Player) Shoot() {
	bullet := &Bullet{
		OwnerID: p.ID,
		X:       p.X + math.Cos(p.Angle)*PlayerRadius,
		Y:       p.Y + math.Sin(p.Angle)*PlayerRadius,
		DX:      math.Cos(p.Angle) * BulletSpeed,
		DY:      math.Sin(p.Angle) * BulletSpeed,
	}
	p.Bullets = append(p.Bullets, bullet)
}

func (b *Bullet) Update() {
	b.X += b.DX
	b.Y += b.DY
}

func (b *Bullet) OutOfBounds() bool {
	return b.X < 0 || b.X > ScreenWidth || b.Y < 0 || b.Y > ScreenHeight
}

func collidesWithObstacles(x, y, radius float64, obstacles []*Obstacle) bool {
	for _, obstacle := range obstacles {
		if circleRectCollision(x, y, radius, obstacle) {
			return true
		}
	}
	return false
}

func circleRectCollision(cx, cy, radius float64, rect *Obstacle) bool {
	closestX := math.Max(rect.X, math.Min(cx, rect.X+rect.Width))
	closestY := math.Max(rect.Y, math.Min(cy, rect.Y+rect.Height))
	dx := cx - closestX
	dy := cy - closestY
	return (dx*dx + dy*dy) < (radius * radius)
}

func bulletHitsObstacle(b *Bullet, obstacles []*Obstacle) bool {
	for _, obstacle := range obstacles {
		if b.X >= obstacle.X && b.X <= obstacle.X+obstacle.Width && b.Y >= obstacle.Y && b.Y <= obstacle.Y+obstacle.Height {
			return true
		}
	}
	return false
}

func (g *Game) Update() error {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.player.Update(g.obstacles)
	g.checkBulletCollisions()
	g.sendPlayerUpdate()
	return nil
}

func (g *Game) checkBulletCollisions() {
	for _, otherPlayer := range g.players {
		if otherPlayer.Health <= 0 || otherPlayer.ID == g.player.ID {
			continue
		}
		for i := len(g.player.Bullets) - 1; i >= 0; i-- {
			bullet := g.player.Bullets[i]
			if bullet.OwnerID == g.player.ID && distance(bullet.X, bullet.Y, otherPlayer.X, otherPlayer.Y) < PlayerRadius+BulletRadius {
				otherPlayer.Health -= 20
				if otherPlayer.Health < 0 {
					otherPlayer.Health = 0
				}
				g.sendEvent(EventTypePlayerHit, PlayerHit{VictimID: otherPlayer.ID, Damage: 20})
				g.player.Bullets = append(g.player.Bullets[:i], g.player.Bullets[i+1:]...)
			}
		}
	}
}

func distance(x1, y1, x2, y2 float64) float64 {
	return math.Hypot(x2-x1, y2-y1)
}

func (g *Game) Draw(screen *ebiten.Image) {
	for _, obstacle := range g.obstacles {
		ebitenutil.DrawRect(screen, obstacle.X, obstacle.Y, obstacle.Width, obstacle.Height, color.RGBA{80, 80, 80, 255})
	}

	ebitenutil.DrawCircle(screen, g.player.X, g.player.Y, PlayerRadius, color.RGBA{0, 255, 0, 255})
	ebitenutil.DebugPrint(screen, fmt.Sprintf("Health: %d", g.player.Health))

	for _, bullet := range g.player.Bullets {
		ebitenutil.DrawCircle(screen, bullet.X, bullet.Y, BulletRadius, color.RGBA{0, 255, 255, 255})
	}

	for _, player := range g.players {
		clr := color.RGBA{255, 0, 0, 255}
		if player.Health <= 0 {
			clr = color.RGBA{100, 100, 100, 255}
		}
		ebitenutil.DrawCircle(screen, player.X, player.Y, PlayerRadius, clr)
		ebitenutil.DebugPrintAt(screen, fmt.Sprintf("%s: %d HP", player.ID, player.Health), int(player.X-20), int(player.Y-30))

		for _, bullet := range player.Bullets {
			ebitenutil.DrawCircle(screen, bullet.X, bullet.Y, BulletRadius, color.RGBA{255, 255, 0, 255})
		}
	}
}

func (g *Game) Layout(_, _ int) (int, int) {
	return ScreenWidth, ScreenHeight
}

func (g *Game) sendPlayerUpdate() {
	update := PlayerUpdate{
		ID:      g.player.ID,
		X:       g.player.X,
		Y:       g.player.Y,
		Angle:   g.player.Angle,
		Health:  g.player.Health,
		Bullets: g.player.Bullets,
	}
	g.sendEvent(EventTypePlayerUpdate, update)
}

func (g *Game) sendEvent(eventType EventType, data interface{}) {
	event := Event{Type: eventType}
	eventData, err := json.Marshal(data)
	if err != nil {
		log.Println("Error marshaling event data:", err)
		return
	}
	event.Data = eventData

	message, err := json.Marshal(event)
	if err != nil {
		log.Println("Error marshaling event:", err)
		return
	}

	if _, err := g.conn.Write(append(message, '\n')); err != nil {
		log.Println("Error sending event:", err)
	}
}

func (g *Game) listenForUpdates() {
	reader := bufio.NewReader(g.conn)
	for {
		msg, err := reader.ReadString('\n')
		if err != nil {
			log.Println("Connection lost:", err)
			return
		}

		var event Event
		if err := json.Unmarshal([]byte(msg), &event); err != nil {
			log.Println("Error unmarshaling event:", err)
			continue
		}

		switch event.Type {
		case EventTypePlayerUpdate:
			var update PlayerUpdate
			if err := json.Unmarshal(event.Data, &update); err != nil {
				log.Println("Error unmarshaling PlayerUpdate:", err)
				continue
			}

			if update.ID == g.player.ID {
				continue // Skip self updates
			}

			g.mu.Lock()
			player, exists := g.players[update.ID]
			if !exists {
				player = NewPlayer(update.ID, update.X, update.Y)
				g.players[update.ID] = player
			}
			player.X = update.X
			player.Y = update.Y
			player.Angle = update.Angle
			player.Health = update.Health
			player.Bullets = update.Bullets
			g.mu.Unlock()

		case EventTypePlayerHit:
			var hit PlayerHit
			if err := json.Unmarshal(event.Data, &hit); err != nil {
				log.Println("Error unmarshaling PlayerHit:", err)
				continue
			}

			g.mu.Lock()
			if player, exists := g.players[hit.VictimID]; exists {
				player.Health -= hit.Damage
				if player.Health < 0 {
					player.Health = 0
				}
			}
			if hit.VictimID == g.player.ID {
				g.player.Health -= hit.Damage
			}
			g.mu.Unlock()
		}
	}
}

func startServer() {
	listener, err := net.Listen("tcp", ServerPort)
	if err != nil {
		log.Fatal("Failed to start server:", err)
	}
	defer listener.Close()
	log.Println("Server running on", ServerPort)

	clients := make(map[net.Conn]bool)
	var mu sync.Mutex

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Println("Connection error:", err)
			continue
		}

		mu.Lock()
		clients[conn] = true
		mu.Unlock()

		go func(c net.Conn) {
			reader := bufio.NewReader(c)
			for {
				msg, err := reader.ReadString('\n')
				if err != nil {
					log.Println("Client disconnected:", err)
					mu.Lock()
					delete(clients, c)
					mu.Unlock()
					return
				}

				mu.Lock()
				for client := range clients {
					if client != c {
						if _, writeErr := client.Write([]byte(msg)); writeErr != nil {
							log.Println("Error sending update to client:", writeErr)
						}
					}
				}
				mu.Unlock()
			}
		}(conn)
	}
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "server" {
		startServer()
		return
	}

	if len(os.Args) < 3 {
		fmt.Println("Usage: go run main.go <player_id> <server_ip:port>")
		return
	}

	playerID := os.Args[1]
	serverAddr := os.Args[2]

	conn, err := net.Dial("tcp", serverAddr)
	if err != nil {
		log.Fatal("Failed to connect to server:", err)
	}
	defer conn.Close()

	game := &Game{
		player:    NewPlayer(playerID, ScreenWidth/2, ScreenHeight/2),
		players:   make(map[string]*Player),
		obstacles: NewObstacles(),
		conn:      conn,
	}

	go game.listenForUpdates()

	ebiten.SetWindowSize(ScreenWidth, ScreenHeight)
	ebiten.SetWindowTitle("2D Multiplayer Top-Down Shooter with Obstacles")
	if err := ebiten.RunGame(game); err != nil {
		log.Fatal(err)
	}
}
