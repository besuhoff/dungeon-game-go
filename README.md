# Multiplayer Shooter Game Server

A real-time authoritative multiplayer shooter game server written in Go with WebSocket support.

## Features

- **Authoritative Server Architecture**: All game logic runs on the server to prevent cheating
- **Real-time Multiplayer**: WebSocket-based communication for low-latency gameplay
- **Binary Protocol Support**: Optional Protocol Buffers encoding for 60% bandwidth reduction (see [Binary Protocol section](docs/binary-protocol.md))
- **Core Game Mechanics**:
  - Player movement with rotation-based controls (forward/backward in facing direction)
  - Lives system with invulnerability after taking damage
  - Bullet recharge system (6 bullets, recharge over time)
  - Shooting mechanics with fire rate limiting
  - Hit detection and collision system with sliding collision resolution
  - Health and scoring system with monetary rewards
  - Enemy AI with patrol and shooting behavior
  - Procedural wall generation in chunks
  - Power-ups: Aid kits (heal) and Night vision goggles
  - Map boundaries with chunk-based world generation
- **60 FPS Game Loop**: Smooth server-side physics and updates
- **Scalable Design**: Concurrent client handling with goroutines

## Project Structure

```
dungeon-game-go/
├── main.go                          # Server entry point
├── go.mod                           # Go module definition
├── internal/
│   ├── game/
│   │   └── engine.go               # Game logic and physics
│   ├── server/
│   │   └── server.go               # WebSocket server and client handling
│   └── types/
│       ├── types.go                # Game entities (Player, Bullet, etc.)
│       └── messages.go             # Client-server message protocol
```

## Prerequisites

- Go 1.21 or higher
- Git (for dependency management)

## Installation

1. Clone or navigate to the project directory:

```bash
cd /Users/spereverziev/project/walknhit/dungeon-game-go
```

2. Install dependencies:

```bash
go mod download
```

## Running the Server

Start the server:

```bash
go run main.go
```

The server will start on `http://localhost:8080`

## API Endpoints

- **WebSocket (JSON)**: `ws://localhost:8080/ws` - Game connection with JSON protocol
- **WebSocket (Binary)**: `ws://localhost:8080/ws?protocol=binary` - Game connection with Protocol Buffers (60% less bandwidth)
- **Health Check**: `http://localhost:8080/health` - Server health status

For details on binary protocol usage, see [Binary Protocol](docs/binary-protocol.md).

## Client-Server Protocol

### Client → Server Messages

#### Connect

```json
{
  "type": "connect",
  "payload": {
    "username": "PlayerName"
  }
}
```

#### Player Input

```json
{
  "type": "input",
  "payload": {
    "forward": true,
    "backward": false,
    "left": false,
    "right": true,
    "direction": 1.5708
  }
}
```

- `direction`: Player facing direction in radians

#### Shoot

```json
{
  "type": "shoot",
  "payload": {
    "direction": 0.785398
  }
}
```

### Server → Client Messages

#### Game State (Broadcast every frame)

```json
{
  "type": "gameState",
  "payload": {
    "players": {
      "player-id": {
        "id": "uuid",
        "username": "PlayerName",
        "position": { "x": 100, "y": 200 },
        "velocity": { "x": 0, "y": 0 },
        "lives": 5,
        "score": 3,
        "money": 150.0,
        "kills": 15,
        "rotation": 90.0,
        "bulletsLeft": 4,
        "nightVisionTimer": 0,
        "isAlive": true
      }
    },
    "bullets": {
      "bullet-id": {
        "id": "uuid",
        "position": { "x": 150, "y": 220 },
        "velocity": { "x": 420, "y": 0 },
        "ownerId": "player-id",
        "damage": 1
      }
    },
    "walls": {
      "wall-id": {
        "id": "uuid",
        "position": { "x": 500, "y": 500 },
        "width": 30,
        "height": 250,
        "orientation": "vertical"
      }
    },
    "enemies": {
      "enemy-id": {
        "id": "uuid",
        "position": { "x": 520, "y": 600 },
        "rotation": 90.0,
        "lives": 1,
        "wallId": "wall-id",
        "isDead": false
      }
    },
    "bonuses": {
      "bonus-id": {
        "id": "uuid",
        "position": { "x": 300, "y": 400 },
        "type": "aid_kit"
      }
    },
    "timestamp": 1234567890
  }
}
```

#### Player Join

```json
{
  "type": "playerJoin",
  "payload": {
    "player": {
      /* player object */
    }
  }
}
```

#### Player Leave

```json
{
  "type": "playerLeave",
  "payload": {
    "playerId": "uuid"
  }
}
```

## Game Configuration

Key constants can be modified in `internal/types/types.go`:

```go
// Player constants
PlayerLives               = 5        // Starting lives
PlayerSpeed               = 300.0    // Units per second
PlayerSize                = 24.0     // Collision size
PlayerRotationSpeed       = 180.0    // Degrees per second
PlayerShootDelay          = 0.2      // Seconds between shots
PlayerMaxBullets          = 6        // Max bullets before reload
PlayerBulletRechargeTime  = 1.0      // Seconds per bullet recharge
PlayerBulletSpeed         = 420.0    // Bullet velocity
PlayerInvulnerabilityTime = 1.0      // Seconds after hit

// Enemy constants
EnemySpeed            = 120.0    // Patrol speed
EnemySize             = 24.0     // Collision size
EnemyLives            = 1        // Health
EnemyShootDelay       = 1.0      // Seconds between shots
EnemyBulletSpeed      = 240.0    // Bullet velocity
EnemyReward           = 10.0     // Money dropped
EnemyDropChance       = 0.3      // 30% bonus drop chance

// Bonus constants
AidKitHealAmount  = 2        // Lives restored
GogglesActiveTime = 20.0     // Seconds of night vision

// World constants
MapWidth  = 10000.0    // World width
MapHeight = 10000.0    // World height
ChunkSize = 800.0      // Chunk generation size
TorchRadius = 200.0    // Vision radius
```

## Building for Production

Build an executable:

```bash
go build -o game-server main.go
```

Run the compiled binary:

```bash
./game-server
```

## Testing

### Test Binary Protocol

```bash
# Run server
go run main.go

# In another terminal, run test client with binary protocol
go run cmd/test-client/main.go -binary

# Or test with JSON protocol (default)
go run cmd/test-client/main.go
```

The test client will connect, send player input, and display received game states.

## Development

### Adding New Features

1. **Game Logic**: Modify `internal/game/engine.go`
2. **Network Protocol**: Update message types in `internal/types/messages.go`
3. **Server Behavior**: Adjust `internal/server/server.go`

### Testing

Run tests (when implemented):

```bash
go test ./...
```

## Client Implementation Example

Here's a basic JavaScript client example:

```javascript
const ws = new WebSocket("ws://localhost:8080/ws");

ws.onopen = () => {
  // Connect with username
  ws.send(
    JSON.stringify({
      type: "connect",
      payload: { username: "MyPlayer" },
    })
  );
};

ws.onmessage = (event) => {
  const message = JSON.parse(event.data);

  if (message.type === "gameState") {
    // Update game rendering with message.payload
    updateGame(message.payload);
  }
};

// Send player input
function sendInput(keys, mouseAngle) {
  ws.send(
    JSON.stringify({
      type: "input",
      payload: {
        forward: keys.w,
        backward: keys.s,
        left: keys.a,
        right: keys.d,
        direction: mouseAngle,
      },
    })
  );
}

// Shoot
function shoot(angle) {
  ws.send(
    JSON.stringify({
      type: "shoot",
      payload: { direction: angle },
    })
  );
}
```

## Architecture Notes

### Authoritative Server

- All game state is maintained on the server
- Client inputs are processed server-side
- Prevents client-side cheating
- Server validates all actions (movement, shooting, collisions)

### Concurrency

- Each client connection runs in its own goroutine
- Game engine uses mutex locks for thread-safe state access
- Broadcast messages are queued and sent asynchronously

### Performance

- 60 FPS game loop (16ms tick rate)
- Efficient collision detection with spatial checks
- Delta-time based physics for consistent movement

## Future Enhancements

- [ ] Client-side prediction and interpolation
- [ ] Lag compensation
- [ ] Multiple game rooms/lobbies
- [ ] Spectator mode
- [ ] Power-ups and different weapon types
- [ ] Respawn system
- [ ] Persistent player stats
- [ ] Anti-cheat measures
- [ ] Match-making system

## License

MIT

## Contributing

Feel free to submit issues and pull requests!
