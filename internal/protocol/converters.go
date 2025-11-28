package protocol

import (
	"github.com/besuhoff/dungeon-game-go/internal/types"
)

// ToProtoVector2 converts types.Vector2 to proto Vector2
func ToProtoVector2(v types.Vector2) *Vector2 {
	return &Vector2{
		X: v.X,
		Y: v.Y,
	}
}

// FromProtoVector2 converts proto Vector2 to types.Vector2
func FromProtoVector2(v *Vector2) types.Vector2 {
	if v == nil {
		return types.Vector2{}
	}
	return types.Vector2{
		X: v.X,
		Y: v.Y,
	}
}

// ToProtoPlayer converts types.Player to proto Player
func ToProtoPlayer(p *types.Player) *Player {
	if p == nil {
		return nil
	}
	return &Player{
		Id:               p.ID,
		Username:         p.Username,
		Position:         ToProtoVector2(p.Position),
		Velocity:         ToProtoVector2(p.Velocity),
		Lives:            int32(p.Lives),
		Score:            int32(p.Score),
		Money:            p.Money,
		Kills:            int32(p.Kills),
		Rotation:         p.Rotation,
		BulletsLeft:      int32(p.BulletsLeft),
		NightVisionTimer: p.NightVisionTimer,
		IsAlive:          p.IsAlive,
	}
}

// ToProtoBullet converts types.Bullet to proto Bullet
func ToProtoBullet(b *types.Bullet) *Bullet {
	if b == nil {
		return nil
	}
	return &Bullet{
		Id:       b.ID,
		Position: ToProtoVector2(b.Position),
		Velocity: ToProtoVector2(b.Velocity),
		OwnerId:  b.OwnerID,
		Damage:   int32(b.Damage),
	}
}

// ToProtoWall converts types.Wall to proto Wall
func ToProtoWall(w *types.Wall) *Wall {
	if w == nil {
		return nil
	}
	return &Wall{
		Id:          w.ID,
		Position:    ToProtoVector2(w.Position),
		Width:       w.Width,
		Height:      w.Height,
		Orientation: w.Orientation,
	}
}

// ToProtoEnemy converts types.Enemy to proto Enemy
func ToProtoEnemy(e *types.Enemy) *Enemy {
	if e == nil {
		return nil
	}
	return &Enemy{
		Id:       e.ID,
		Position: ToProtoVector2(e.Position),
		Rotation: e.Rotation,
		Lives:    int32(e.Lives),
		WallId:   e.WallID,
		IsDead:   e.IsDead,
	}
}

// ToProtoBonus converts types.Bonus to proto Bonus
func ToProtoBonus(b *types.Bonus) *Bonus {
	if b == nil {
		return nil
	}
	return &Bonus{
		Id:       b.ID,
		Position: ToProtoVector2(b.Position),
		Type:     b.Type,
	}
}

// ToProtoGameState converts types.GameState to proto GameState
func ToProtoGameState(gs types.GameState) *GameState {
	protoPlayers := make(map[string]*Player)
	for k, v := range gs.Players {
		protoPlayers[k] = ToProtoPlayer(v)
	}

	protoBullets := make(map[string]*Bullet)
	for k, v := range gs.Bullets {
		protoBullets[k] = ToProtoBullet(v)
	}

	protoWalls := make(map[string]*Wall)
	for k, v := range gs.Walls {
		protoWalls[k] = ToProtoWall(v)
	}

	protoEnemies := make(map[string]*Enemy)
	for k, v := range gs.Enemies {
		protoEnemies[k] = ToProtoEnemy(v)
	}

	protoBonuses := make(map[string]*Bonus)
	for k, v := range gs.Bonuses {
		protoBonuses[k] = ToProtoBonus(v)
	}

	return &GameState{
		Players:   protoPlayers,
		Bullets:   protoBullets,
		Walls:     protoWalls,
		Enemies:   protoEnemies,
		Bonuses:   protoBonuses,
		Timestamp: gs.Timestamp,
	}
}

// FromProtoInput converts proto InputMessage to types.InputPayload
func FromProtoInput(input *InputMessage) types.InputPayload {
	if input == nil {
		return types.InputPayload{}
	}
	return types.InputPayload{
		Forward:   input.Forward,
		Backward:  input.Backward,
		Left:      input.Left,
		Right:     input.Right,
	}
}

// FromProtoShoot converts proto ShootMessage to types.ShootPayload
func FromProtoShoot(shoot *ShootMessage) types.ShootPayload {
	if shoot == nil {
		return types.ShootPayload{}
	}
	return types.ShootPayload{
		Direction: shoot.Direction,
	}
}

// FromProtoConnect converts proto ConnectMessage to types.ConnectPayload
func FromProtoConnect(connect *ConnectMessage) types.ConnectPayload {
	if connect == nil {
		return types.ConnectPayload{}
	}
	return types.ConnectPayload{
		Username: connect.Username,
	}
}
