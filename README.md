# Redis Portal

A high-performance TCP proxy/bridge application written in Go forwards traffic to target redis server.

## Why???

The story is: i ran into a situation that i need to be able to access a redis container on a vps from my local machine. But the issue is that redis container has no password protection, and no port exposed to public.
And i really really don't want to change any config, then restart the container, because it could interupt other services using it.
So i came up with this stupid idea of creating an app to only serve one single purpose.

## Features

- **TCP Proxy**: Forwards all traffic between clients and target server
- **Redis-Compatible Authentication**: Supports both client and target Redis authentication
- **Bidirectional Communication**: Supports full duplex communication
- **Graceful Shutdown**: Proper signal handling and connection cleanup
- **Structured Logging**: Beautiful, readable logging with emojis and visual separators
- **Command Line Interface**: Easy configuration with both long and short flags
- **Docker Support**: Environment detection and Docker-specific warnings
- **Network Discovery**: Automatic local and external IP detection

## Usage

### Command Line Arguments

```bash
go run main.go -target <target_host:port> [options]
```

**Required Arguments:**
- `-target, -t`: Target server address (e.g., `redis:6379`, `localhost:3306`)

**Optional Arguments:**
- `-listen, -l`: Local address to listen on (default: `:8379`)
- `-username, -u`: Client authentication username (default: empty)
- `-password, -p`: Client authentication password (default: random generated)
- `-target-username, -tu`: Target Redis username for authentication
- `-target-password, -tp`: Target Redis password for authentication

### Examples

#### Basic Redis Proxy (No Authentication)
```bash
go run main.go -target redis:6379
# or with short flags
go run main.go -t redis:6379
```

#### Redis Proxy with Client Authentication
```bash
go run main.go -target redis:6379 -username admin -password secret123
# or with short flags
go run main.go -t redis:6379 -u admin -p secret123
```

#### Redis Proxy with Target Authentication
```bash
go run main.go -target redis:6379 -target-username myuser -target-password mypass
# or with short flags
go run main.go -t redis:6379 -tu myuser -tp mypass
```

#### Full Authentication (Client + Target)
```bash
go run main.go -target redis:6379 -username portaluser -password portalpass -target-username redisuser -target-password redispass
# or with short flags
go run main.go -t redis:6379 -u portaluser -p portalpass -tu redisuser -tp redispass
```

#### Custom Listen Address
```bash
go run main.go -target redis:6379 -listen :3307
# or with short flags
go run main.go -t redis:6379 -l :3307
```

## How It Works

1. **Client Connection**: Client connects to the TCP portal
2. **Client Authentication**: If configured, client authenticates using Redis AUTH command
3. **Target Connection**: Portal connects to the target server
4. **Target Authentication**: If configured, portal authenticates with target using Redis AUTH command
5. **Data Forwarding**: All data is bidirectionally forwarded between client and target
6. **Connection Management**: Connections are properly cleaned up when closed

## Authentication Protocol

The authentication uses Redis-compatible AUTH commands:

### Client Authentication
1. Client connects to portal
2. Client sends: `AUTH username password` (Redis 6.0+) or `AUTH password` (Redis < 6.0)
3. Portal responds: `+OK` or `-ERR WRONGPASS invalid username-password pair`
4. If authentication succeeds, data forwarding begins

### Target Authentication
1. Portal connects to target Redis server
2. Portal sends: `AUTH username password` or `AUTH password`
3. Target responds: `+OK` or error
4. If authentication succeeds, data forwarding begins

## Building and Running

### Build the application
```bash
go build -o redis-portal main.go
```

### Run the binary
```bash
# Basic usage
./redis-portal -target redis:6379

# With authentication
./redis-portal -target redis:6379 -username admin -password secret123

# With short flags
./redis-portal -t redis:6379 -u admin -p secret123
```

## Docker Usage

### Using the Docker Image

You can use the pre-built Docker image `luuhai48/redis-portal` to run the application without building it locally.

#### Basic Usage
```bash
# Pull and run the latest version
docker run -p 8379:8379 --network=your-backend-network luuhai48/redis-portal:latest -target redis:6379

# Or use a specific version
docker run -p 8379:8379 --network=your-backend-network luuhai48/redis-portal:0.0.1 -target redis:6379
```

#### With Authentication
```bash
# Client authentication only
docker run -p 8379:8379 --network=your-backend-network luuhai48/redis-portal:latest -target redis:6379 -username admin -password secret123

# Target authentication only
docker run -p 8379:8379 --network=your-backend-network luuhai48/redis-portal:latest -target redis:6379 -target-password redispass

# Both client and target authentication
docker run -p 8379:8379 --network=your-backend-network luuhai48/redis-portal:latest -target redis:6379 -username portaluser -password portalpass -target-username redisuser -target-password redispass
```

#### Custom Listen Port
```bash
# Listen on port 3307 instead of default 8379
docker run -p 3307:3307 --network=your-backend-network luuhai48/redis-portal:latest -target redis:6379 -listen :3307
```

## Testing with Redis

### Using Docker Compose (Recommended)

1. Start Redis services:
   ```bash
   docker-compose up -d
   ```

2. Start the TCP portal:
   ```bash
   # No authentication
   go run main.go -target localhost:6379
   
   # With target authentication
   go run main.go -target localhost:6380 -target-password secretpassword
   
   # With both client and target authentication
   go run main.go -target localhost:6380 -password mypass -target-password secretpassword
   ```

3. Connect using redis-cli:
   ```bash
   # Connect to the portal
   redis-cli -p 8379
   
   # If client authentication is required
   AUTH mypass
   
   # Now you can use Redis commands
   SET test "hello world"
   GET test
   ```

### Manual Testing

1. Start Redis server:
   ```bash
   redis-server --port 6379
   ```

2. Start the TCP portal:
   ```bash
   go run main.go -target localhost:6379
   ```

3. Test with redis-cli:
   ```bash
   redis-cli -p 8379
   SET test "hello world"
   GET test
   ```

## License

This project is open source and available under the MIT License.
