package player

import (
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"math"
	"math/rand"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"shooter/game"
	"shooter/utils"
)

const (
	MaxHealth               = 100
	PlayerSpeed             = 1.0
	PlayerSprintSpeedFactor = 1.7
	BulletSpeed             = 80.0
	PlayerRadius            = 10.0
	BulletRadius            = 3.0
	ShootCooldown           = 200 * time.Millisecond
)

var PlayerSprite = utils.MustLoadImage("assets/survivor-idle_rifle_0.png")

type EventType string

const (
	EventTypePlayerUpdate EventType = "player_update"
	EventTypePlayerHit    EventType = "player_hit"
)

type Event struct {
	Type EventType       `json:"type"`
	Data json.RawMessage `json:"data"`
}

type Player struct {
	ID       string    `json:"id"`
	X        float64   `json:"x"`
	Y        float64   `json:"y"`
	Angle    float64   `json:"angle"`
	Health   int       `json:"health"`
	Bullets  []*Bullet `json:"bullets"`
	lastShot time.Time `json:"-"`
	sprite   *ebiten.Image
}

func (player Player) SpriteBounds() image.Rectangle {
	return player.sprite.Bounds()
}

func (p *Player) HitBox() game.Object {
	// TODO: this is crap, create new object with centered x,y and use wh
	dx := float64(p.SpriteBounds().Dx()) * 0.25
	dy := float64(p.SpriteBounds().Dy()) * 0.25
	return game.Object{Walls: game.Rect(
		p.X-dx/2,
		p.Y-dy/2,
		dx,
		dy,
	)}
}

func NewPlayer(id string, x, y float64) *Player {
	return &Player{
		ID:       id,
		X:        x,
		Y:        y,
		Angle:    0,
		Health:   MaxHealth,
		Bullets:  []*Bullet{},
		lastShot: time.Time{},
		sprite:   PlayerSprite,
	}
}

type Bullet struct {
	OwnerID   string  `json:"owner_id"`
	X         float64 `json:"x"`
	Y         float64 `json:"y"`
	EndX      float64 `json:"end_x"`
	EndY      float64 `json:"end_y"`
	Direction float64 `json:"direction"`
	Velocity  float64 `json:"velocity"`
}

func (p *Player) UpdateOnObstacle() {
	moveX, moveY := 0.0, 0.0

	// Move horizontally and check collision
	p.X += moveX
	// if collidesWithObstacles(p.X, p.Y, PlayerRadius, obstacles) {
	// p.X -= moveX // Revert horizontal movement if collides
	// }
	//
	// // Move vertically and check collision
	p.Y += moveY
	// if collidesWithObstacles(p.X, p.Y, PlayerRadius, obstacles) {
	// p.Y -= moveY // Revert vertical movement if collides
	// }
}

// func (p *Player) Update() {
// 	if p.Health <= 0 {
// 		return
// 	}
//
// 	moveX, moveY := 0.0, 0.0
// 	movementSpeed := PlayerSpeed
//
// 	if ebiten.IsKeyPressed(ebiten.KeyShiftLeft) {
// 		movementSpeed *= PlayerSprintSpeedFactor
// 	}
//
// 	if ebiten.IsKeyPressed(ebiten.KeyW) {
// 		moveY -= movementSpeed
// 	}
// 	if ebiten.IsKeyPressed(ebiten.KeyS) {
// 		moveY += movementSpeed
// 	}
// 	if ebiten.IsKeyPressed(ebiten.KeyA) {
// 		moveX -= movementSpeed
// 	}
// 	if ebiten.IsKeyPressed(ebiten.KeyD) {
// 		moveX += movementSpeed
// 	}
//
// 	p.X += moveX
// 	p.Y += moveY
// 	// Update aiming angle
// 	mx, my := ebiten.CursorPosition()
// 	dx, dy := float64(mx)-p.X, float64(my)-p.Y
// 	p.Angle = math.Atan2(dy, dx)
//
// 	// Shooting
// 	if ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) && time.Since(p.lastShot) > ShootCooldown {
// 		p.Shoot()
// 		p.lastShot = time.Now()
// 	}
//
// 	// Update bullets
// 	for i := len(p.Bullets) - 1; i >= 0; i-- {
// 		p.Bullets[i].Update()
// 		if p.Bullets[i].OutOfBounds(1600, 900) {
// 			p.Bullets = append(p.Bullets[:i], p.Bullets[i+1:]...)
// 		}
// 	}
// }

func (p *Player) Update(hitsObstacle bool) {
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

	p.X += moveX
	if hitsObstacle {
		p.X -= moveX // Revert horizontal movement if collides
	}

	// // Move vertically and check collision
	p.Y += moveY
	if hitsObstacle {
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
		if p.Bullets[i].OutOfBounds(1600, 900) {
			p.Bullets = append(p.Bullets[:i], p.Bullets[i+1:]...)
		}
	}
}

func (p *Player) Draw(screen *ebiten.Image) {
	vector.DrawFilledCircle(screen, float32(p.X), float32(p.Y), PlayerRadius, color.RGBA{0, 255, 0, 255}, true)
	ebitenutil.DebugPrintAt(screen, "WASD: move", 160, 0)
	ebitenutil.DebugPrintAt(screen, fmt.Sprintf("TPS: %0.2f", ebiten.ActualTPS()), 51, 51)
	ebitenutil.DebugPrintAt(screen, fmt.Sprintf("FPS: %0.2f", ebiten.ActualFPS()), 51, 61)

	// TODO: separate player package for logic and ui
	bounds := p.SpriteBounds()
	opPlayer := &ebiten.DrawImageOptions{}

	hw := float64(bounds.Dx() / 2)
	hh := float64(bounds.Dy() / 2)

	opPlayer.GeoM.Translate(-hw, -hh)
	opPlayer.GeoM.Scale(0.25, 0.25)
	opPlayer.GeoM.Rotate(p.Angle)
	// op.GeoM.Translate(hw, hh)
	opPlayer.GeoM.Translate(p.X, p.Y)

	screen.DrawImage(p.sprite, opPlayer)
	vector.DrawFilledCircle(screen, float32(p.X), float32(p.Y), PlayerRadius, color.RGBA{0, 255, 0, 255}, false)
	vector.StrokeLine(screen, float32(p.HitBox().Walls[0].X1), float32(p.HitBox().Walls[0].Y1), float32(p.HitBox().Walls[0].X2), float32(p.HitBox().Walls[0].Y2), 1.0, color.White, false)
	vector.StrokeLine(screen, float32(p.HitBox().Walls[1].X1), float32(p.HitBox().Walls[1].Y1), float32(p.HitBox().Walls[1].X2), float32(p.HitBox().Walls[1].Y2), 1.0, color.White, false)
	vector.StrokeLine(screen, float32(p.HitBox().Walls[2].X1), float32(p.HitBox().Walls[2].Y1), float32(p.HitBox().Walls[2].X2), float32(p.HitBox().Walls[2].Y2), 1.0, color.White, false)
	vector.StrokeLine(screen, float32(p.HitBox().Walls[3].X1), float32(p.HitBox().Walls[3].Y1), float32(p.HitBox().Walls[3].X2), float32(p.HitBox().Walls[3].Y2), 1.0, color.White, false)
	ebitenutil.DebugPrint(screen, fmt.Sprintf("Health: %d", p.Health))
}

func (p *Player) Shoot() {
	angleRecoil := (rand.Float64() - 0.5) / 10
	bullet := &Bullet{
		OwnerID:   p.ID,
		X:         p.X,
		Y:         p.Y,
		EndX:      p.X + math.Cos(p.Angle+angleRecoil)*BulletSpeed,
		EndY:      p.Y + math.Sin(p.Angle+angleRecoil)*BulletSpeed,
		Direction: p.Angle + angleRecoil,
		Velocity:  BulletSpeed,
	}
	p.Bullets = append(p.Bullets, bullet)
}

func (b *Bullet) Update() {
	dx := math.Cos(b.Direction) * b.Velocity
	dy := math.Sin(b.Direction) * b.Velocity
	b.X += dx
	b.Y += dy
	b.EndX += dx
	b.EndY += dy
}

func (b *Bullet) OutOfBounds(width, height float64) bool {
	return b.X < 0 || b.X > width || b.Y < 0 || b.Y > height
}

func (b *Bullet) Line() game.Line {
	return game.Line{
		X1: b.X,
		Y1: b.Y,
		X2: b.EndX,
		Y2: b.EndY,
	}
}

func (b *Bullet) Draw(screen *ebiten.Image) {
	// vector.DrawFilledCircle(screen, float32(b.X), float32(b.Y), 1, color.White, false)
	// TODO: bulled line dissapears before hitbox

	// vector.StrokeLine(screen, float32(b.X), float32(b.Y), float32(b.EndX+25*math.Cos(b.Direction)), float32(b.EndY+25*math.Sin(b.Direction)), 1.7, color.White, false)
	vector.StrokeLine(screen, float32(b.X), float32(b.Y), float32(b.EndX), float32(b.EndY), 1.7, color.White, false)
}
