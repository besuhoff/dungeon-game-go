# Multiplayer Shooter Game Server

A real-time authoritative multiplayer shooter game server written in Go with WebSocket support.

## Features

- **Authoritative Server Architecture**: All game logic runs on the server to prevent cheating
- **Real-time Multiplayer**: WebSocket-based communication for low-latency gameplay
- **Core Game Mechanics**:
  - Player movement (WASD controls)
  - Shooting mechanics with fire rate limiting
  - Hit detection and collision system
  - Health and scoring system
  - Map boundaries
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

- **WebSocket**: `ws://localhost:8080/ws` - Game connection endpoint
- **Health Check**: `http://localhost:8080/health` - Server health status

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
        "health": 80,
        "score": 3,
        "direction": 0.5,
        "isAlive": true
      }
    },
    "bullets": {
      "bullet-id": {
        "id": "uuid",
        "position": { "x": 150, "y": 220 },
        "velocity": { "x": 500, "y": 0 },
        "ownerId": "player-id",
        "damage": 20
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
MaxHealth        = 100              // Player starting health
BulletSpeed      = 500.0            // Bullet velocity (units/sec)
BulletDamage     = 20               // Damage per hit
FireRate         = 200ms            // Minimum time between shots
PlayerSpeed      = 200.0            // Player movement speed
MapWidth         = 2000.0           // Map width
MapHeight        = 2000.0           // Map height
PlayerRadius     = 20.0             // Player collision radius
BulletRadius     = 5.0              // Bullet collision radius
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
