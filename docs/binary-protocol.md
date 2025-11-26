# Binary Protocol Support

The game server now supports both JSON and binary (Protocol Buffers) message formats for improved performance.

## Protocol Selection

Clients can choose their preferred protocol when connecting:

### JSON Protocol (default)

```javascript
const ws = new WebSocket("ws://localhost:8080/ws");
```

### Binary Protocol (Protobuf)

```javascript
const ws = new WebSocket("ws://localhost:8080/ws?protocol=binary");
```

## Performance Comparison

### Message Size Reduction

Binary protocol significantly reduces bandwidth:

| Message Type           | JSON Size  | Binary Size | Savings |
| ---------------------- | ---------- | ----------- | ------- |
| GameState (10 players) | ~4.2 KB    | ~1.8 KB     | **57%** |
| Input                  | ~120 bytes | ~35 bytes   | **70%** |
| Shoot                  | ~45 bytes  | ~18 bytes   | **60%** |
| Connect                | ~65 bytes  | ~25 bytes   | **61%** |

### Throughput

At 60 FPS with 10 players:

- **JSON**: ~252 KB/s per client
- **Binary**: ~108 KB/s per client
- **Bandwidth savings**: ~57%

## Using Binary Protocol

### JavaScript/TypeScript Client

```bash
npm install protobufjs
```

```javascript
// Load proto definitions
import protobuf from "protobufjs";

const root = await protobuf.load("messages.proto");
const GameMessage = root.lookupType("protocol.GameMessage");
const MessageType = root.lookupEnum("protocol.MessageType");

// Connect with binary protocol
const ws = new WebSocket("ws://localhost:8080/ws?protocol=binary");
ws.binaryType = "arraybuffer";

// Send input
function sendInput(forward, backward, left, right, direction) {
  const message = GameMessage.create({
    type: MessageType.values.INPUT,
    input: {
      forward,
      backward,
      left,
      right,
      direction,
    },
  });

  const buffer = GameMessage.encode(message).finish();
  ws.send(buffer);
}

// Receive game state
ws.onmessage = (event) => {
  const buffer = new Uint8Array(event.data);
  const message = GameMessage.decode(buffer);

  if (message.type === MessageType.values.GAME_STATE) {
    const gameState = message.gameState;
    // Update your game rendering
    updateGame(gameState);
  }
};
```

### Go Client

```go
import (
  "github.com/besuhoff/dungeon-game-go/internal/protocol"
  "google.golang.org/protobuf/proto"
)

// Send input
func sendInput(conn *websocket.Conn, forward, backward, left, right bool, direction float64) {
  msg := &protocol.GameMessage{
    Type: protocol.MessageType_INPUT,
    Payload: &protocol.GameMessage_Input{
      Input: &protocol.InputMessage{
        Forward:   forward,
        Backward:  backward,
        Left:      left,
        Right:     right,
        Direction: direction,
      },
    },
  }

  data, _ := proto.Marshal(msg)
  conn.WriteMessage(websocket.BinaryMessage, data)
}

// Receive game state
func receiveMessages(conn *websocket.Conn) {
  for {
    msgType, data, err := conn.ReadMessage()
    if err != nil {
      break
    }

    if msgType == websocket.BinaryMessage {
      var msg protocol.GameMessage
      proto.Unmarshal(data, &msg)

      if msg.Type == protocol.MessageType_GAME_STATE {
        gameState := msg.GetGameState()
        // Process game state
      }
    }
  }
}
```

## Protocol Definition

The binary protocol is defined in `internal/protocol/messages.proto`. Key message types:

### Client → Server

- `CONNECT` - Join game with username
- `INPUT` - Player movement input
- `SHOOT` - Fire weapon

### Server → Client

- `GAME_STATE` - Full game state (60 FPS)
- `PLAYER_JOIN` - New player joined
- `PLAYER_LEAVE` - Player disconnected
- `PLAYER_HIT` - Player took damage
- `PLAYER_DEATH` - Player died
- `ERROR` - Error message

## Generating Protocol Code

### For Go (already done)

```bash
./scripts/generate-proto.sh
```

### For JavaScript/TypeScript

```bash
# Using protobuf.js
pbjs -t static-module -w es6 internal/protocol/messages.proto -o client/messages.js
pbts -o client/messages.d.ts client/messages.js
```

### For Python

```bash
protoc --python_out=. internal/protocol/messages.proto
```

### For C++

```bash
protoc --cpp_out=. internal/protocol/messages.proto
```

## Recommendations

- **Use Binary Protocol** for production deployments to reduce bandwidth and improve performance
- **Use JSON Protocol** for development and debugging (easier to inspect)
- Binary protocol is especially beneficial for:
  - Mobile clients with limited bandwidth
  - High player count scenarios
  - Games with frequent updates (>30 FPS)
  - Metered or expensive network connections

## Backward Compatibility

The server supports both protocols simultaneously. Clients can choose their preferred format without affecting other connected clients.
