package types

// MessageType represents different types of messages
type MessageType string

const (
	// Client -> Server
	MsgTypeConnect    MessageType = "connect"
	MsgTypeInput      MessageType = "input"
	MsgTypeShoot      MessageType = "shoot"
	MsgTypeDisconnect MessageType = "disconnect"
	
	// Server -> Client
	MsgTypeGameState  MessageType = "gameState"
	MsgTypePlayerJoin MessageType = "playerJoin"
	MsgTypePlayerLeave MessageType = "playerLeave"
	MsgTypePlayerHit  MessageType = "playerHit"
	MsgTypePlayerDeath MessageType = "playerDeath"
	MsgTypeError      MessageType = "error"
)

// Message is the base message structure
type Message struct {
	Type    MessageType `json:"type"`
	Payload interface{} `json:"payload,omitempty"`
}

// ConnectPayload for connect messages
type ConnectPayload struct {
	Username string `json:"username"`
}

// InputPayload for player input
type InputPayload struct {
	Forward   bool    `json:"forward"`
	Backward  bool    `json:"backward"`
	Left      bool    `json:"left"`
	Right     bool    `json:"right"`
	Direction float64 `json:"direction"` // player facing direction in degrees
}

// ShootPayload for shooting
type ShootPayload struct {
	Direction float64 `json:"direction"`
}

// PlayerJoinPayload for player join notifications
type PlayerJoinPayload struct {
	Player *Player `json:"player"`
}

// PlayerLeavePayload for player leave notifications
type PlayerLeavePayload struct {
	PlayerID string `json:"playerId"`
}

// PlayerHitPayload for hit notifications
type PlayerHitPayload struct {
	PlayerID   string `json:"playerId"`
	AttackerID string `json:"attackerId"`
	Damage     int    `json:"damage"`
	NewHealth  int    `json:"newHealth"`
}

// PlayerDeathPayload for death notifications
type PlayerDeathPayload struct {
	PlayerID   string `json:"playerId"`
	KillerID   string `json:"killerId"`
}

// ErrorPayload for error messages
type ErrorPayload struct {
	Message string `json:"message"`
}
