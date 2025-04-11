# IO Game Backend

This is a WebSocket-based multiplayer real-time game backend. Players can control characters to move within the game area and use weapons to attack other players.

## Features

- Real-time multiplayer interaction
- Weapon system and damage calculation
- Player health management
- Collision detection
- Boundary constraints

## Requirements

- Docker
- Or Go 1.19+

## Build and Run with Docker

1. Build Docker image:

```bash
docker build -t iogame .
```

2. Run Docker container:

```bash
docker run -p 8080:8080 iogame
```

