# TCP Portal

A high-performance TCP proxy/bridge application written in Go that provides Redis-compatible authentication and forwards traffic to target servers with comprehensive logging and graceful shutdown capabilities.

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
- `-listen, -l`: Local address to listen on (default: `:8080`)
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
go build -o tcp-portal main.go
```

### Run the binary
```bash
# Basic usage
./tcp-portal -target redis:6379

# With authentication
./tcp-portal -target redis:6379 -username admin -password secret123

# With short flags
./tcp-portal -t redis:6379 -u admin -p secret123
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
   redis-cli -p 8080
   
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
   redis-cli -p 8080
   SET test "hello world"
   GET test
   ```

## Enhanced Features

### Structured Logging
The application provides beautiful, readable logging with:
- 🚀 Visual separators and emojis for different log types
- 📋 Clear configuration display
- 🔐 Authentication status indicators
- 🌐 Network information with local and external IPs
- 🔗 Connection tracking and status updates

### Graceful Shutdown
- Responds to SIGINT (Ctrl+C) and SIGTERM signals
- Waits for all active connections to close before exiting
- Proper cleanup of resources

### Docker Support
- Automatic detection of Docker environment
- Warnings for problematic configurations (localhost binding in Docker)
- Port mapping reminders

### Network Discovery
- Automatic detection of local IP address
- External IP discovery (with timeout to avoid blocking)
- Clear display of all access methods

## Security Considerations

- Uses constant-time comparison for password checking to prevent timing attacks
- Redis-compatible authentication protocol
- Authentication happens per connection
- No persistent sessions or tokens
- Consider using TLS for production environments

## Error Handling

The application handles various error conditions:
- Authentication failures (both client and target)
- Target server connection failures
- Network I/O errors
- Connection timeouts
- Protocol errors

All errors are logged with appropriate context and structured information.

## Logging

The application provides comprehensive structured logging including:
- 🚀 Server startup and configuration
- 📋 Environment detection and network information
- 🔐 Authentication attempts and results
- 🔗 Connection events and status updates
- ❌ Error conditions with detailed context
- ⚠️ Warnings for configuration issues
- ℹ️ General information and status updates

### Log Levels
- **INFO**: General information, connections, successful operations
- **WARN**: Configuration warnings, authentication failures
- **ERROR**: Connection errors, protocol errors, critical failures

## Docker Compose Setup

The project includes a `docker-compose.yml` file with Redis services for testing:

### Services
- **redis-no-auth** (port 6379): Redis without authentication
- **redis-with-auth** (port 6380): Redis with password authentication

### Usage
```bash
# Start all Redis services
docker-compose up -d

# Stop all services
docker-compose down

# View logs
docker-compose logs
```

### Testing Different Scenarios
```bash
# Test with no authentication
go run main.go -target localhost:6379

# Test with target password authentication
go run main.go -target localhost:6380 -target-password secretpassword

# Test with both client and target authentication
go run main.go -target localhost:6380 -password mypass -target-password secretpassword
```

## License

This project is open source and available under the MIT License.
