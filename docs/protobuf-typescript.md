# Protocol Buffers TypeScript Usage

## Installation

Install the protobuf-ts runtime in your frontend project:

```bash
npm install @protobuf-ts/runtime
# or
yarn add @protobuf-ts/runtime
```

## Generate TypeScript Code

Run the generation script:

```bash
./scripts/generate-proto.sh
```

This creates TypeScript files in `internal/protocol/` with full JSON support.

## Usage Examples

### Decoding Binary Messages from WebSocket

```typescript
import {
  GameMessage,
  GameStateDelta,
  GameState,
} from "./internal/protocol/messages";

const ws = new WebSocket(
  "ws://localhost:8080/ws?token=YOUR_TOKEN&protocol=binary"
);
ws.binaryType = "arraybuffer";

ws.onmessage = (event) => {
  const bytes = new Uint8Array(event.data);
  const message = GameMessage.fromBinary(bytes);

  switch (message.type) {
    case MessageType.GAME_STATE_DELTA:
      const delta = message.payload.value as GameStateDelta;

      // Access added players
      for (const [id, player] of Object.entries(delta.addedPlayers)) {
        console.log("New player:", player.username, player.position);
      }

      // Access updated players
      for (const [id, player] of Object.entries(delta.updatedPlayers)) {
        updatePlayerPosition(id, player.position);
      }

      // Handle removed entities
      delta.removedPlayers.forEach((id) => removePlayer(id));
      break;

    case MessageType.GAME_STATE:
      const state = message.payload.value as GameState;
      // Full state sync
      break;
  }
};
```

### Creating Messages to Send

```typescript
import {
  GameMessage,
  MessageType,
  InputMessage,
} from "./internal/protocol/generated/messages";

// Create input message
const input = InputMessage.create({
  forward: true,
  backward: false,
  left: false,
  right: true,
});

const message = GameMessage.create({
  type: MessageType.INPUT,
  payload: { oneofKind: "input", input },
});

// Encode to binary
const bytes = GameMessage.toBinary(message);
ws.send(bytes);
```

### JSON Conversion (fromObject/toJSON)

The generated code includes JSON support:

```typescript
import { GameState } from "./internal/protocol/generated/messages";

// From JSON object
const jsonData = {
  players: {
    player1: {
      id: "player1",
      username: "Alice",
      position: { x: 100, y: 200 },
      lives: 5,
    },
  },
  timestamp: Date.now(),
};

const gameState = GameState.fromJson(jsonData);

// To JSON
const json = GameState.toJson(gameState);
console.log(json);

// To JavaScript object
const obj = GameState.toObject(gameState);
```

### Working with Delta Updates

```typescript
// Apply delta to existing state
function applyDelta(currentState: Map<string, Player>, delta: GameStateDelta) {
  // Add new players
  for (const [id, player] of Object.entries(delta.addedPlayers)) {
    currentState.set(id, player);
  }

  // Update existing players
  for (const [id, player] of Object.entries(delta.updatedPlayers)) {
    currentState.set(id, player);
  }

  // Remove players
  delta.removedPlayers.forEach((id) => currentState.delete(id));
}
```

## Benefits

✅ **Type Safety**: Full TypeScript types for all messages  
✅ **JSON Support**: Built-in `fromJson()`, `toJson()`, `toObject()` methods  
✅ **Binary Efficiency**: 60-70% smaller than JSON over WebSocket  
✅ **Validation**: Automatic field validation and type checking  
✅ **Easy Debugging**: Convert to/from JSON for logging

## Alternative: Using JSON Protocol

If you prefer JSON over binary:

```typescript
const ws = new WebSocket("ws://localhost:8080/ws?token=YOUR_TOKEN"); // no protocol=binary

ws.onmessage = (event) => {
  const message = JSON.parse(event.data);

  if (message.type === "gameStateDelta") {
    const delta = message.payload;
    // Use delta data directly as JSON
  }
};

// Send JSON
ws.send(
  JSON.stringify({
    type: "input",
    payload: { forward: true, left: true },
  })
);
```
