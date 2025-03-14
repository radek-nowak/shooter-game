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
	"sort"
	"sync"

	"shooter/game"
	"shooter/player"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

const (
	ScreenWidth  = 1600
	ScreenHeight = 900
	ServerPort   = ":8080"

	PlayerRadius = 10.0
	BulletRadius = 3.0

	// raycasting
	RayCount       = 120    // Number of rays casted for visibility
	RayLength      = 1600.0 // Maximum ray length
	ObstacleBorder = 2.0
)

type Obstacle struct {
	X      float64
	Y      float64
	Width  float64
	Height float64
}

type PlayerUpdate struct {
	ID      string           `json:"id"`
	X       float64          `json:"x"`
	Y       float64          `json:"y"`
	Angle   float64          `json:"angle"`
	Health  int              `json:"health"`
	Bullets []*player.Bullet `json:"bullets"`
}

type PlayerHit struct {
	VictimID string `json:"victim_id"`
	Damage   int    `json:"damage"`
}

type Game struct {
	player    *player.Player
	players   map[string]*player.Player
	obstacles []*Obstacle
	Objects   []game.Object
	conn      net.Conn
	mu        sync.Mutex
}

func NewObstacles() []*Obstacle {
	return []*Obstacle{
		{X: 200, Y: 150, Width: 100, Height: 200},
		{X: 500, Y: 300, Width: 150, Height: 100},
	}
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

func (g *Game) castRays(cx, cy float64, objects []game.Object) []game.Line {
	rayLength := math.Hypot(float64(ScreenWidth), float64(ScreenHeight)) // something large enough to reach all objects

	rays := []game.Line{}

	for _, obj := range objects {
		// Cast two rays per point
		for _, p := range obj.Points() {
			l := game.Line{cx, cy, p[0], p[1]}
			angle := l.Angle()

			for _, offset := range []float64{-0.001, 0.001} {
				points := [][2]float64{}
				ray := game.NewRay(cx, cy, rayLength, angle+offset)

				// Unpack all objects
				for _, o := range objects {
					for _, wall := range o.Walls {
						if px, py, ok := game.Intersection(ray, wall); ok {
							points = append(points, [2]float64{px, py})
						}
					}
				}

				// Find the point closest to start of ray
				min := math.Inf(1)
				minI := 0
				for i, p := range points {
					d2 := (cx-p[0])*(cx-p[0]) + (cy-p[1])*(cy-p[1])
					if d2 < min {
						min = d2
						minI = i
					}
				}
				if minI < len(points) {
					rays = append(rays, game.Line{X1: cx, Y1: cy, X2: points[minI][0], Y2: points[minI][1]})
				}
			}
		}
	}
	sort.Slice(rays, func(i int, j int) bool {
		return rays[i].Angle() < rays[j].Angle()
	})
	return rays
}

func (g *Game) Update() error {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.player.Update()
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
			// if bullet.OwnerID == g.player.ID && distance(bullet.X, bullet.Y, otherPlayer.X, otherPlayer.Y) < PlayerRadius+BulletRadius {
			// if LineIntersectsCircle(bullet.X, bullet.Y, 1000*math.Cos(bullet.Direction), 1000*math.Sin(bullet.Direction), otherPlayer.X, otherPlayer.Y, 10) {
			for _, l := range otherPlayer.HitBox().Walls {
				if _, _, intersects := game.Intersection(l, bullet.Line()); intersects {
					otherPlayer.Health -= 20
					if otherPlayer.Health < 0 {
						otherPlayer.Health = 0
					}
					g.sendEvent(player.EventTypePlayerHit, PlayerHit{VictimID: otherPlayer.ID, Damage: 20})
					// g.player.Bullets = append(g.player.Bullets[:i], g.player.Bullets[i+1:]...)
					continue
				}
			}
		}
	}
}

func LineIntersectsCircle(x1, y1, x2, y2, cx, cy, radius float64) bool {
	// Vector from start to end of the Line
	// y1 = -y1
	// y2 = -y2
	dx := x2 - x1
	dy := y2 - y1

	// Vector from start of the Line to the circle center
	fx := x1 - cx
	fy := y1 - cy

	a := dx*dx + dy*dy
	b := 2 * (fx*dx + fy*dy)
	c := (fx*fx + fy*fy) - radius*radius

	// ebitenutil.DebugPrintAt(screen, fmt.Sprintf("%s: %d HP", player.ID, player.Health), int(player.X-20), int(player.Y-30))

	discriminant := b*b - 4*a*c
	if discriminant < 0 {
		// No intersection
		return false
	}

	discriminant = math.Sqrt(discriminant)

	// Find the two points of intersection, t0 and t1
	t0 := (-b - discriminant) / (2 * a)
	t1 := (-b + discriminant) / (2 * a)

	// Check if either of the intersection points is within the Line segment
	return (t0 >= 0 && t0 <= 1) || (t1 >= 0 && t1 <= 1)
}

func distance(x1, y1, x2, y2 float64) float64 {
	return math.Hypot(x2-x1, y2-y1)
}

var (
	shadowImage   = ebiten.NewImage(ScreenWidth, ScreenHeight)
	triangleImage = ebiten.NewImage(ScreenWidth, ScreenHeight)
	bgImage       *ebiten.Image
)

func rayVertices(x1, y1, x2, y2, x3, y3 float64) []ebiten.Vertex {
	return []ebiten.Vertex{
		{DstX: float32(x1), DstY: float32(y1), SrcX: 0, SrcY: 0, ColorR: 1, ColorG: 1, ColorB: 1, ColorA: 1},
		{DstX: float32(x2), DstY: float32(y2), SrcX: 0, SrcY: 0, ColorR: 1, ColorG: 1, ColorB: 1, ColorA: 1},
		{DstX: float32(x3), DstY: float32(y3), SrcX: 0, SrcY: 0, ColorR: 1, ColorG: 1, ColorB: 1, ColorA: 1},
	}
}

func (g *Game) Draw(screen *ebiten.Image) {
	// for _, obstacle := range g.obstacles {
	// 	ebitenutil.DrawRect(screen, obstacle.X, obstacle.Y, obstacle.Width, obstacle.Height, color.RGBA{80, 80, 80, 255})
	// }

	// laser
	// laserLength := float64(ScreenWidth)
	// laserEndX := g.player.X + math.Cos(g.player.Angle)*laserLength
	// laserEndY := g.player.Y + math.Sin(g.player.Angle)*laserLength
	// ebitenutil.DrawLine(screen, g.player.X, g.player.Y, laserEndX, laserEndY, color.RGBA{255, 0, 0, 255})
	// vector.StrokeLine(screen, float32(g.player.X), float32(g.player.Y), float32(laserEndX), float32(laserEndY), 1.0, color.RGBA{255, 0, 0, 255}, true)
	shadowImage.Fill(color.Black)

	rays := g.castRays(g.player.X, g.player.Y, g.Objects)

	opts := &ebiten.DrawTrianglesOptions{}
	opts.Address = ebiten.AddressRepeat
	opts.Blend = ebiten.BlendDestinationOut

	screen.DrawImage(bgImage, nil)

	for _, bullet := range g.player.Bullets {
		vector.DrawFilledCircle(screen, float32(bullet.X), float32(bullet.Y), BulletRadius, color.RGBA{0, 255, 255, 255}, false)
		bullet.Draw(screen)
	}

	for _, p := range g.players {
		clr := color.RGBA{255, 0, 0, 255}
		if p.Health <= 0 {
			clr = color.RGBA{100, 100, 100, 255}
		}
		// ebitenutil.DrawCircle(screen, player.X, player.Y, PlayerRadius, clr)
		p.Draw(screen)
		vector.DrawFilledCircle(screen, float32(p.X), float32(p.Y), PlayerRadius, clr, false)
		// ebitenutil.DebugPrintAt(screen, fmt.Sprintf("%s: %d HP", player.ID, player.Health), int(player.X-20), int(player.Y-30))

		for _, bullet := range p.Bullets {
			bullet.Draw(screen)
			// vector.DrawFilledCircle(screen, float32(bullet.X), float32(bullet.Y), BulletRadius, color.RGBA{255, 255, 0, 255}, true)
		}
	}

	for i, ray := range rays {
		nextLine := rays[(i+1)%len(rays)]

		v := rayVertices(g.player.X, g.player.Y, nextLine.X2, nextLine.Y2, ray.X2, ray.Y2)
		shadowImage.DrawTriangles(v, []uint16{0, 1, 2}, triangleImage, opts)
	}

	// for _, ray := range rays {
	// 	vector.StrokeLine(screen, float32(ray.X1), float32(ray.Y1), float32(ray.X2), float32(ray.Y2), 1, color.RGBA{255, 255, 0, 100}, true)
	// }

	op := &ebiten.DrawImageOptions{}
	// op.ColorScale.ScaleAlpha(0.9)
	// op.ColorScale.ScaleAlpha(0.7)
	screen.DrawImage(shadowImage, op)

	// Draw obstacles
	for _, obs := range g.Objects {
		for _, w := range obs.Walls {
			vector.StrokeLine(screen, float32(w.X1), float32(w.Y1), float32(w.X2), float32(w.Y2), 1, color.RGBA{255, 0, 0, 255}, true)
		}
	}

	// Draw player
	g.player.Draw(screen)
	for _, b := range g.player.Bullets {
		b.Draw(screen)
	}

	// vector.DrawFilledCircle(screen, float32(g.player.X), float32(g.player.Y), PlayerRadius, color.RGBA{0, 255, 0, 255}, true)
	// ebitenutil.DebugPrintAt(screen, "WASD: move", 160, 0)
	// ebitenutil.DebugPrintAt(screen, fmt.Sprintf("TPS: %0.2f", ebiten.ActualTPS()), 51, 51)
	// ebitenutil.DebugPrintAt(screen, fmt.Sprintf("FPS: %0.2f", ebiten.ActualFPS()), 51, 61)
	// ebitenutil.DebugPrintAt(screen, fmt.Sprintf("Rays: 2*%d", len(rays)/2), 200, 222)
	//
	// // TODO: separate player package for logic and ui
	// bounds := g.player.SpriteBounds()
	// opPlayer := &ebiten.DrawImageOptions{}
	//
	// hw := float64(bounds.Dx() / 2)
	// hh := float64(bounds.Dy() / 2)
	//
	// opPlayer.GeoM.Translate(-hw, -hh)
	// opPlayer.GeoM.Scale(0.25, 0.25)
	// opPlayer.GeoM.Rotate(g.player.Angle)
	// // op.GeoM.Translate(hw, hh)
	// opPlayer.GeoM.Translate(g.player.X, g.player.Y)
	//
	// screen.DrawImage(g.player.sprite, opPlayer)
	// vector.DrawFilledCircle(screen, float32(g.player.X), float32(g.player.Y), PlayerRadius, color.RGBA{0, 255, 0, 255}, false)
	// ebitenutil.DebugPrint(screen, fmt.Sprintf("Health: %d", g.player.Health))
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
	g.sendEvent(player.EventTypePlayerUpdate, update)
}

func (g *Game) sendEvent(eventType player.EventType, data interface{}) {
	// TODO: player creates events, which games sends
	event := player.Event{Type: eventType}
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

		var event player.Event
		if err := json.Unmarshal([]byte(msg), &event); err != nil {
			log.Println("Error unmarshaling event:", err)
			continue
		}

		switch event.Type {
		case player.EventTypePlayerUpdate:
			var update PlayerUpdate
			if err := json.Unmarshal(event.Data, &update); err != nil {
				log.Println("Error unmarshaling PlayerUpdate:", err)
				continue
			}

			if update.ID == g.player.ID {
				continue // Skip self updates
			}

			g.mu.Lock()
			p, exists := g.players[update.ID]
			if !exists {
				p = player.NewPlayer(update.ID, update.X, update.Y)
				g.players[update.ID] = p
			}
			p.X = update.X
			p.Y = update.Y
			p.Angle = update.Angle
			p.Health = update.Health
			p.Bullets = update.Bullets
			g.mu.Unlock()

		case player.EventTypePlayerHit:
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

const padding = 20

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

	bgImage, _, _ = ebitenutil.NewImageFromFile("./aa.png")

	triangleImage.Fill(color.White)

	g := &Game{
		player:    player.NewPlayer(playerID, ScreenWidth/2, ScreenHeight/2),
		players:   make(map[string]*player.Player),
		obstacles: []*Obstacle{},
		Objects: []game.Object{{
			Walls: game.Rect(
				padding,
				padding,
				ScreenWidth-2*padding,
				ScreenHeight-2*padding,
			),
		}, {
			Walls: game.Rect(
				ScreenWidth/2-50,
				ScreenHeight/2+50,
				100, 100,
			),
		}},
		conn: conn,
		mu:   sync.Mutex{},
	}

	// for i := 0; i < 50; i++ {
	// 	game.objects = append(game.objects, object{rect(200+float64(50*i), 50+float64(25*i), 30, 20)})
	// }

	go g.listenForUpdates()

	ebiten.SetWindowSize(ScreenWidth, ScreenHeight)
	ebiten.SetWindowTitle("2D Multiplayer Top-Down Shooter with Obstacles")
	if err := ebiten.RunGame(g); err != nil {
		log.Fatal(err)
	}
}
