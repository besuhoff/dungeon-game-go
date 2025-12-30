package server

import (
	"fmt"
	"log"
	"time"

	"github.com/besuhoff/dungeon-game-go/internal/protocol"
	"github.com/gorilla/websocket"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// WebsocketClient represents a connected client
type WebsocketClient struct {
	ID          string
	UserID      primitive.ObjectID // MongoDB User ID
	Username    string
	SessionID   string // Game session ID
	SessionName string
	Conn        *websocket.Conn
	Send        chan []byte
	Server      *GameServer
	UseBinary   bool // Whether client prefers binary protocol
}

// Client methods
func (c *WebsocketClient) readPump() {
	defer func() {
		c.Server.unregister <- c
	}()

	c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.Conn.SetPongHandler(func(string) error {
		c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		messageType, message, err := c.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}

		var msg protocol.GameMessage
		// Handle binary or text messages
		if messageType == websocket.BinaryMessage {
			c.unmarshalProtoMessage(message, &msg)
		} else {
			c.unmarshalJSONMessage(message, &msg)
		}

		c.handleMessage(&msg)
	}
}

func (c *WebsocketClient) writePump() {
	ticker := time.NewTicker(54 * time.Second)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.Send:
			c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			// Send as binary or text based on client preference
			msgType := websocket.TextMessage
			if c.UseBinary {
				msgType = websocket.BinaryMessage
			}

			if err := c.Conn.WriteMessage(msgType, message); err != nil {
				return
			}

		case <-ticker.C:
			c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (c *WebsocketClient) unmarshalProtoMessage(data []byte, msg *protocol.GameMessage) error {
	if err := proto.Unmarshal(data, msg); err != nil {
		return fmt.Errorf("unmarshaling proto message: %v", err)
	}
	return nil
}

func (c *WebsocketClient) unmarshalJSONMessage(data []byte, msg *protocol.GameMessage) error {
	if err := protojson.Unmarshal(data, msg); err != nil {
		return fmt.Errorf("unmarshaling JSON message: %v", err)
	}
	return nil
}

func (c *WebsocketClient) handleMessage(msg *protocol.GameMessage) {
	// Get session engine
	c.Server.mu.RLock()
	session, exists := c.Server.sessions[c.SessionID]
	c.Server.mu.RUnlock()

	if !exists {
		log.Printf("Session %s not found for client %s", c.SessionID, c.UserID.Hex())
		return
	}

	switch msg.Type {
	case protocol.MessageType_INPUT:
		if input := msg.GetInput(); input != nil {
			payload := protocol.FromProtoInput(input)
			session.Engine.UpdatePlayerInput(c.UserID.Hex(), payload)
		}
	case protocol.MessageType_PLAYER_RESPAWN:
		if respawn := msg.GetPlayerRespawn(); respawn != nil {
			session.Engine.RespawnPlayer(c.UserID.Hex())
		}
	}
}

func (c *WebsocketClient) SendJSON(msg *protocol.GameMessage) {
	data, err := protojson.Marshal(msg)
	if err != nil {
		log.Printf("Error marshaling message: %v", err)
		return
	}
	select {
	case c.Send <- data:
	default:
		// Buffer full
	}
}

func (c *WebsocketClient) SendBinary(msg *protocol.GameMessage) {
	data, err := proto.Marshal(msg)
	if err != nil {
		log.Printf("Error marshaling binary message: %v", err)
		return
	}
	select {
	case c.Send <- data:
	default:
		// Buffer full
	}
}

func (c *WebsocketClient) SendGameStateDelta(delta *protocol.GameStateDeltaMessage) {
	msg := &protocol.GameMessage{
		Type: protocol.MessageType_GAME_STATE_DELTA,
		Payload: &protocol.GameMessage_GameStateDelta{
			GameStateDelta: delta,
		},
	}

	if c.UseBinary {
		c.SendBinary(msg)
	} else {
		c.SendJSON(msg)
	}
}
