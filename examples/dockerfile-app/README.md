# Dockerfile Example App

This is a simple example of a Dockerfile-based app for Atmosphere.

## Structure

- `Dockerfile` - Defines the container image
- `index.html` - Static HTML content served by nginx

## Deployment

### Using API

```bash
# 1. Create the app
curl -X POST http://localhost:3000/api/v1/apps \
  -H "Content-Type: application/json" \
  -d '{
    "name": "my-static-site",
    "deployment_type": "manual",
    "build_type": "dockerfile",
    "domain": "mysite.example.com",
    "port": 80
  }'

# 2. Upload Dockerfile
curl -X POST http://localhost:3000/api/v1/apps/my-static-site/files \
  -F "path=Dockerfile" \
  -F "content=@Dockerfile"

# 3. Upload index.html
curl -X POST http://localhost:3000/api/v1/apps/my-static-site/files \
  -F "path=index.html" \
  -F "content=@index.html"

# 4. Deploy
curl -X POST http://localhost:3000/api/v1/apps/my-static-site/deploy

# 5. Check deployment status
curl http://localhost:3000/api/v1/apps/my-static-site
```

## Customization

Edit `index.html` to customize the content, or replace nginx with any other web server in the Dockerfile.
