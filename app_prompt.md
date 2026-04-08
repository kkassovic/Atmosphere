You are a senior DevOps and backend architect. Build an MVP self-hosted deployment platform from scratch, inspired by Coolify/Dokploy, but much simpler, more stable, and focused only on Dockerfile and Docker Compose deployments.

The project goal:
Create a lightweight deployment system for Linux servers that can:
1. deploy apps from GitHub repositories using deployment keys
2. deploy apps by pasting/uploading raw project files manually
3. use Traefik as the reverse proxy
4. automatically route incoming traffic to running containers
5. manage only Dockerfile-based apps and docker-compose based apps
6. prepare a clean foundation for adding a GUI later

Important:
- Keep it simple
- Prioritize reliability over features
- Do not add Kubernetes, Swarm, Nomad, or other orchestration layers
- Do not support anything except Dockerfile and Docker Compose deployments
- The system should be CLI/backend-first
- Code should be modular so a GUI can be added later
- Use production-friendly patterns, but avoid overengineering

Project requirements
====================

1. Core architecture
- Build a small backend service that manages deployments on one Linux server
- The backend should expose a clean internal structure and optionally a minimal REST API
- Store metadata about deployments, apps, domains, repositories, and status
- Use simple persistence:
  - SQLite for MVP is acceptable
  - structure code so DB can later be swapped to MySQL/PostgreSQL
- Use Docker Engine on the host
- Use Traefik in a dedicated Docker container as the reverse proxy
- Traefik should automatically discover/reroute traffic to deployed application containers
- Prefer Docker labels for Traefik integration
- Support HTTPS with Let's Encrypt if domain is configured
- Design so custom domains can be assigned per app

2. Deployment modes
Implement two deployment modes:

A. GitHub deployment
- Deploy from private or public GitHub repositories
- Use SSH deployment keys
- For each app:
  - repository URL
  - branch
  - optional subdirectory
  - deployment key
- Clone/pull repository into a local workspace
- Detect whether the project uses:
  - Dockerfile
  - docker-compose.yml
  - docker-compose.yaml
  - compose.yml
  - compose.yaml
- Let user specify custom docker-copose or Dockerfile
- Build and deploy accordingly

B. Manual file deployment
- Allow creating an app by copy/pasting files into the platform
- At minimum support:
  - Dockerfile
  - docker-compose.yml / compose.yml
  - .env
  - any additional files needed for build context
- Store these files in a local workspace directory on disk
- Then build and deploy them the same way as GitHub deployments

3. Deployment behavior
- Each app should have:
  - unique app name / slug
  - deployment type (github/manual)
  - domain(s)
  - environment variables
  - build type (dockerfile/compose)
  - status
  - last deployment logs
- Deployments should:
  - create isolated project directories
  - build images safely
  - start containers
  - stop/replace old version when redeploying
- Support restart, stop, redeploy, remove
- Keep logs of build/deployment results
- Clearly separate app workspace, runtime config, and metadata
- Avoid shell hacks where possible; use structured code
- However, practical shell execution is acceptable for Docker and Git commands

4. Reverse proxy / Traefik
- Use Traefik as a container managed by this platform
- Traefik should:
  - listen on ports 80 and 443
  - use Docker provider
  - automatically route based on labels
  - support Let's Encrypt
- Generate or provide required Traefik static and dynamic config
- Ensure deployed app containers get proper labels for:
  - router
  - service
  - entrypoints
  - TLS
  - host rules
- If app has no domain yet, support temporary internal/default routing strategy if practical
- Make Traefik setup easy to understand and maintain

5. Installation/bootstrap script
Create a first-install script for Ubuntu server that:
- installs required packages
- installs Docker Engine and Docker Compose plugin
- installs git
- installs curl
- installs jq
- installs s3cmd
- creates required directories
- prepares SSH/deployment key storage
- creates Docker network(s) needed for Traefik and apps
- installs and starts Traefik container
- prepares Let's Encrypt storage with correct permissions
- optionally installs basic firewall rules guidance
- prints post-install summary

The installation script should be:
- idempotent as much as practical
- safe to rerun
- well commented
- intended for Ubuntu 24.04 LTS
- non-interactive as much as possible

6. Suggested tech stack
Choose a backend stack that is simple and maintainable.
Preferred options:
- Go
or
- Node.js with TypeScript

Pick one and explain why briefly in README.
My preference is reliability, low memory usage, and ease of deployment.

7. Project structure
Create a clean repository structure, for example:
- backend source
- install scripts
- traefik config
- app workspaces
- deployment key storage
- docs
- examples
- systemd service file if useful

8. Required deliverables
Generate the full MVP codebase, including:
- backend service code
- installer script
- Traefik configuration
- Docker integration logic
- GitHub deployment-key support
- manual file/project storage support
- SQLite schema/migrations
- README with setup and usage
- sample API routes or CLI commands
- example app deployment definitions
- systemd unit file to run backend service on boot
- .env.example where useful

9. API / CLI expectations
A minimal REST API is acceptable.
At minimum, support operations like:
- create app
- deploy app
- redeploy app
- stop app
- start app
- delete app
- list apps
- view deployment logs
- add/update domains
- add/update env vars
- register GitHub repo config
- upload/save manual files

If you believe CLI-first is better for MVP, you may provide both:
- REST API
- simple CLI wrapper

10. Security expectations
- Store deployment keys securely on disk with strict permissions
- Do not expose secrets in logs
- Sanitize shell arguments
- Validate app names, domains, and paths
- Prevent path traversal in manual file uploads/pasted files
- Assume single-server trusted-admin MVP, but still follow sane security defaults

11. Non-goals
Do NOT implement:
- Kubernetes
- Docker Swarm
- multi-node cluster support
- advanced RBAC
- billing
- OAuth login
- GUI frontend
- metrics dashboards
- autoscaling
- non-Docker runtimes

12. GUI preparation
Even though GUI will come later, design the backend so a GUI can easily call it.
That means:
- clear service layer
- clean API boundaries
- predictable JSON responses
- modular deployment logic

13. Output format
I want you to produce:
1. a brief architecture summary
2. the proposed folder structure
3. the full code for the MVP
4. the install script
5. the README
6. explanation of how deployment flow works
7. explanation of how Traefik routing works
8. explanation of how manual file deployment works
9. explanation of how GitHub deployment-key based deployment works

14. Coding style
- Write production-style code
- Prefer clarity over cleverness
- Add comments where useful
- Use explicit error handling
- Avoid unnecessary dependencies
- Make the code easy for another developer to extend

15. MVP priority order
Implement in this order:
1. installer script
2. Traefik setup
3. backend service skeleton
4. app metadata persistence
5. manual deployment of compose apps
6. manual deployment of Dockerfile apps
7. GitHub repository deployment via SSH deployment key
8. domain assignment and Traefik labels
9. logs and lifecycle commands

If some part is too large for one response, generate the project in logically separated files and continue until complete.

Before coding, first provide:
- chosen stack
- concise architecture
- folder structure
- key implementation decisions

Then generate the codebase.