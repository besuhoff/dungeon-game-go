# REST API Endpoints

This document describes the REST API endpoints available in the Dungeon Game server.

## Authentication Endpoints

### Get Google OAuth URL

```
GET /api/v1/auth/google/url
```

Returns a Google OAuth URL for authentication.

**Response:**

```json
{
  "url": "https://accounts.google.com/o/oauth2/auth?..."
}
```

### Google OAuth Callback

```
GET /api/v1/auth/google/callback?code=...&state=...
```

Handles the Google OAuth callback and returns a JWT token.

**Response:**

```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "user": {
    "id": "...",
    "email": "user@example.com",
    "username": "username",
    "is_active": true
  }
}
```

### Get Current User

```
GET /api/v1/auth/user
Authorization: Bearer <token>
```

Returns information about the currently authenticated user.

**Response:**

```json
{
  "id": "...",
  "email": "user@example.com",
  "username": "username",
  "google_id": "...",
  "is_active": true,
  "current_session": "...",
  "created_at": "2024-01-01T00:00:00Z"
}
```

## Session Endpoints

All session endpoints require authentication via `Authorization: Bearer <token>` header.

### Create Session

```
POST /api/v1/sessions
Authorization: Bearer <token>
Content-Type: application/json

{
  "name": "My Game Session",
  "health": 5,
  "max_players": 4,
  "is_private": false,
  "password": "optional_password"
}
```

Creates a new game session. The authenticated user becomes the host.

**Parameters:**

- `name` (string, required): Session name (1-50 characters)
- `health` (int, optional): Starting health for players
- `max_players` (int, optional): Maximum number of players (default: 4)
- `is_private` (bool, optional): Whether the session requires a password
- `password` (string, optional): Password for private sessions

**Response:** `201 Created`

```json
{
  "id": "...",
  "name": "My Game Session",
  "host": {
    "id": "...",
    "email": "user@example.com",
    "username": "username",
    "is_active": true,
    "created_at": "2024-01-01T00:00:00Z"
  },
  "max_players": 4,
  "is_private": false,
  "world_map": {},
  "shared_objects": {},
  "game_state": {},
  "players": {
    "player_id": {
      "player_id": "...",
      "name": "username",
      "position": { "x": 0, "y": 0, "rotation": 0 },
      "lives": 5,
      "score": 0,
      "money": 0,
      "kills": 0,
      "is_alive": true,
      "is_connected": false
    }
  },
  "created_at": "2024-01-01T00:00:00Z",
  "is_active": true
}
```

### List Active Sessions

```
GET /api/v1/sessions
Authorization: Bearer <token>
```

Returns a list of all active game sessions.

**Response:** `200 OK`

```json
[
  {
    "id": "...",
    "name": "My Game Session",
    "host": {...},
    "max_players": 4,
    "is_private": false,
    "world_map": {...},
    "shared_objects": {...},
    "game_state": {...},
    "players": {...},
    "created_at": "2024-01-01T00:00:00Z",
    "is_active": true
  }
]
```

### Join Session

```
POST /api/v1/sessions/{session_id}/join
Authorization: Bearer <token>
Content-Type: application/json

{
  "password": "optional_password"
}
```

Joins an existing game session.

**Parameters:**

- `session_id` (path): The ID of the session to join
- `password` (body, optional): Password if the session is private

**Response:** `200 OK`
Returns the full session details (same structure as Create Session response).

**Error Responses:**

- `400 Bad Request`: Session is full
- `403 Forbidden`: Invalid password for private session
- `404 Not Found`: Session not found

### Leave Session

```
POST /api/v1/sessions/{session_id}/leave
Authorization: Bearer <token>
```

Leaves a game session. If the host leaves:

- A new host is assigned from remaining players
- If no players remain, the session is deactivated

**Parameters:**

- `session_id` (path): The ID of the session to leave

**Response:** `200 OK`

```json
{
  "message": "Successfully left session"
}
```

## Leaderboard Endpoints

All leaderboard endpoints require authentication via `Authorization: Bearer <token>` header.

### Get Global Leaderboard

```
GET /api/v1/leaderboard/global?limit=100&timeframe=all
Authorization: Bearer <token>
```

Returns the global leaderboard with top players.

**Query Parameters:**

- `limit` (int, optional): Maximum number of entries to return (default: 100)
- `timeframe` (string, optional): Time filter - `all`, `weekly`, or `monthly` (default: `all`)

**Score Calculation:**

- Base score: `player.Score`
- Lives: `player.Lives × 10`
- Kills: `player.Kills × 50`
- Alive bonus: `100` points
- Money: `player.Money`

**Response:** `200 OK`

```json
[
  {
    "username": "player1",
    "score": 550,
    "session_id": "...",
    "created_at": "2024-01-01T00:00:00Z"
  },
  {
    "username": "player2",
    "score": 420,
    "session_id": "...",
    "created_at": "2024-01-01T00:00:00Z"
  }
]
```

### Get User Statistics

```
GET /api/v1/leaderboard/user/{user_id}
Authorization: Bearer <token>
```

Returns detailed statistics for a specific user.

**Parameters:**

- `user_id` (path): The ID of the user

**Response:** `200 OK`

```json
{
  "total_games": 42,
  "highest_score": 850,
  "average_score": 425.5,
  "recent_scores": [
    {
      "score": 550,
      "session_id": "...",
      "created_at": "2024-01-01T00:00:00Z"
    }
  ]
}
```

**Error Responses:**

- `400 Bad Request`: Invalid user ID format
- `404 Not Found`: User not found

## WebSocket Endpoint

### Connect to Game

```
WS /ws?token=<jwt_token>&sessionId=<session_id>&protocol=json
```

Establishes a WebSocket connection for real-time game communication.

**Query Parameters:**

- `token` (required): JWT authentication token
- `sessionId` (optional): Session ID to join (created if not provided)
- `protocol` (optional): `json` or `binary` (default: `json`)

**Message Format (JSON):**

```json
{
  "type": "playerMove",
  "data": {
    "x": 100,
    "y": 200,
    "rotation": 45
  }
}
```

**Binary Protocol:**
Uses Protocol Buffers for efficient binary encoding (57-70% bandwidth reduction).

## Error Responses

All endpoints may return the following error responses:

- `400 Bad Request`: Invalid request data
- `401 Unauthorized`: Missing or invalid authentication token
- `403 Forbidden`: Access denied (e.g., wrong password)
- `404 Not Found`: Resource not found
- `405 Method Not Allowed`: HTTP method not supported
- `500 Internal Server Error`: Server error

## CORS Configuration

All API endpoints support CORS for the configured `FRONTEND_URL`. The following headers are set:

- `Access-Control-Allow-Origin`: Value from `FRONTEND_URL` environment variable
- `Access-Control-Allow-Methods`: `GET, POST, PUT, DELETE, OPTIONS`
- `Access-Control-Allow-Headers`: `Content-Type, Authorization`
- `Access-Control-Allow-Credentials`: `true`

## Rate Limiting

Currently, there are no rate limits implemented. This should be added in production environments.

## Authentication

Most endpoints require JWT authentication. Include the token in the `Authorization` header:

```
Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...
```

Tokens are valid for 8 days after issuance.
