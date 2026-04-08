# Docker Compose Example App

This is an example Node.js application with Redis, demonstrating multi-service deployments with Atmosphere.

## Structure

- `docker-compose.yml` - Defines the services (web + redis)
- `Dockerfile` - Defines the web service image
- `server.js` - Node.js Express application
- `package.json` - Node.js dependencies

## Features

- Node.js web server with Express
- Redis for state/caching
- Page view counter
- Health check endpoint

## Deployment via GitHub

```bash
# Create app pointing to your GitHub repository
curl -X POST http://localhost:3000/api/v1/apps \
  -H "Content-Type: application/json" \
  -d '{
    "name": "my-compose-app",
    "deployment_type": "github",
    "build_type": "compose",
    "domain": "app.example.com",
    "github_repo": "git@github.com:yourusername/your-repo.git",
    "github_branch": "main",
    "deployment_key": "'"$(cat ~/.ssh/deploy_key)"'",
    "env_vars": {
      "NODE_ENV": "production",
      "PORT": "3000"
    }
  }'

# Deploy
curl -X POST http://localhost:3000/api/v1/apps/my-compose-app/deploy
```

## Manual Deployment

```bash
# 1. Create the app
curl -X POST http://localhost:3000/api/v1/apps \
  -H "Content-Type: application/json" \
  -d '{
    "name": "my-compose-app",
    "deployment_type": "manual",
    "build_type": "compose",
    "domain": "app.example.com"
  }'

# 2-5. Upload all files
for file in docker-compose.yml Dockerfile package.json server.js; do
  curl -X POST http://localhost:3000/api/v1/apps/my-compose-app/files \
    -F "path=$file" \
    -F "content=@$file"
done

# 6. Deploy
curl -X POST http://localhost:3000/api/v1/apps/my-compose-app/deploy

# Check logs
curl http://localhost:3000/api/v1/apps/my-compose-app/logs
```

## Environment Variables

The app uses these environment variables:
- `NODE_ENV` - Node environment (default: production)
- `PORT` - Server port (default: 3000)

## Testing Locally

```bash
docker-compose up
```

Visit http://localhost:3000
