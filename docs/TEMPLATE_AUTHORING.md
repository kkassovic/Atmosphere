# Template Authoring Guide

This guide explains how to add new app templates for Atmosphere Phase 1.

## Location

Create each template under:

- `templates/apps/<template-id>/`

Required file:

- `template.json`

Optional additional files:

- `docker-compose.yml`
- `.env.example`
- app config files

All files except `template.json` are copied into the provisioned app workspace.

## Template ID rules

- lowercase letters, numbers, hyphens
- max 64 chars

Example:

- `uptime-kuma`
- `vaultwarden`

## template.json schema

```json
{
  "id": "uptime-kuma",
  "name": "Uptime Kuma",
  "description": "Uptime monitoring dashboard",
  "deployment_type": "manual",
  "build_type": "compose",
  "compose_path": "docker-compose.yml",
  "port": 3001,
  "inputs": [
    {
      "name": "admin_token",
      "label": "Admin Token",
      "type": "secret",
      "required": false,
      "default": "",
      "description": "Optional template input"
    }
  ],
  "default_env": {
    "EXAMPLE_KEY": "{{admin_token}}"
  },
  "metadata": {
    "category": "monitoring"
  }
}
```

## Supported placeholders

In template files and `default_env` values:

- `{{app_name}}`
- `{{domain}}` (first domain from request)
- `{{domains_csv}}` (comma-separated domains)
- any input key from `inputs` (for example `{{db_password}}`)

If a required placeholder value is missing, provisioning fails.

## Add a new template (step-by-step)

1. Create folder `templates/apps/<template-id>/`.
2. Add `template.json`.
3. Add runtime files (`docker-compose.yml`, config files, etc.).
4. Use placeholders for values that should come from request inputs.
5. Start Atmosphere and verify:
   - `GET /api/v1/templates`
   - `GET /api/v1/templates/<template-id>`
6. Provision test app:
   - `POST /api/v1/templates/<template-id>/provision`

## Provision request example

```bash
curl -X POST http://localhost:3000/api/v1/templates/uptime-kuma/provision \
  -H "Content-Type: application/json" \
  -d '{
    "app_name": "kuma-prod",
    "domains": ["status.example.com"],
    "inputs": {},
    "env_vars": {},
    "auto_deploy": true
  }'
```

## Notes

- Keep templates minimal and deterministic.
- Avoid embedding secrets in template files.
- Prefer `inputs` + placeholders for any sensitive or environment-specific values.
- Use `default_env` for sane defaults that users can override via `env_vars` in the provision request.
