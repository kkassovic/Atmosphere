# Templates API Guide

This guide describes Phase 1 app templates in Atmosphere.

Templates are filesystem-backed and loaded from:

- `TEMPLATES_DIR` environment variable
- default: `templates/apps`

Each template must live under:

- `templates/apps/<template-id>/template.json`
- additional files (for example `docker-compose.yml`) are rendered and copied into the new app workspace during provisioning

## Endpoints

### List templates

```bash
curl http://localhost:3000/api/v1/templates | jq
```

### Get template details

```bash
curl http://localhost:3000/api/v1/templates/wordpress | jq
```

### Provision app from template

```bash
curl -X POST http://localhost:3000/api/v1/templates/wordpress/provision \
  -H "Content-Type: application/json" \
  -d '{
    "app_name": "my-wordpress",
    "domains": ["wp.example.com"],
    "inputs": {
      "db_name": "wordpress",
      "db_user": "wordpress",
      "db_password": "change-me",
      "db_root_password": "change-me-root"
    },
    "auto_deploy": true
  }' | jq
```

## Template manifest

`template.json` example:

```json
{
  "id": "uptime-kuma",
  "name": "Uptime Kuma",
  "description": "Uptime monitoring dashboard",
  "deployment_type": "manual",
  "build_type": "compose",
  "compose_path": "docker-compose.yml",
  "port": 3001,
  "inputs": [],
  "default_env": {},
  "metadata": {
    "category": "monitoring"
  }
}
```

## Variable rendering

Template files support placeholders like:

- `{{app_name}}`
- `{{domain}}`
- `{{domains_csv}}`
- any template input key from `inputs`

If a placeholder value is missing, provisioning fails with a validation error.

## Starter templates included

- `wordpress`
- `uptime-kuma`
- `vaultwarden`
- `superset`
- `netdata`
- `home-page`
