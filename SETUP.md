# NeoBase Setup Guide

This guide provides detailed instructions for setting up NeoBase on your local machine or server. Follow the appropriate section based on your deployment needs.

## Table of Contents

- [Prerequisites](#prerequisites)
- [Application Dependencies](#application-dependencies)
  - [Core Dependencies](#core-dependencies)
  - [Supported Databases (for querying)](#supported-databases-for-querying)
  - [Supported LLM Clients](#supported-llm-clients)
- [Setup Options](#setup-options)
- [Manual Setup](#manual-setup)
  - [Frontend/Client Setup](#frontendclient-setup)
- [Docker Compose Setup](#docker-compose-setup)
  - [Local Development Setup](#local-development-setup)
    - [Running Example Databases (Optional)](#running-example-databases-optional)
  - [Server Deployment: Docker Compose](#server-deployment-docker-compose)
  - [Server Deployment: Dokploy](#server-deployment-dokploy)
- [First-Time Setup](#first-time-setup)
- [Troubleshooting](#troubleshooting)

## Prerequisites

Before you begin, ensure you have the following installed:

- **Docker & Docker Compose** - For containerized deployment
- **Go (v1.22+)** - For backend development
- **Node.js (v18+)** - For frontend development
- **OpenAI API Key or Google Gemini API Key** - For AI functionality

## Application Dependencies

NeoBase requires the following services to function properly:

### Core Dependencies

- **MongoDB** - Stores user data, connections, and chat history
- **Redis** - Manages user sessions and database schema caching

### Supported Databases (for querying)

- PostgreSQL
- MySQL
- Yugabyte
- ClickHouse
- MongoDB
- Cassandra (Planned)
- Redis (Planned)
- Neo4j (Planned)

### Supported LLM Clients

- OpenAI (Any chat completion model)
- Google Gemini (Any chat completion model)
- Anthropic Claude (Planned)
- Ollama (Planned)

## Setup Options

You can set up NeoBase in several ways:

1. [Manual Setup](#manual-setup) - Run each component separately
2. [Docker Compose Setup](#docker-compose-setup) - Run everything with Docker Compose

## Manual Setup

### Frontend/Client Setup

1. Navigate to the client directory:

   ```bash
   cd client/
   ```

2. Create an environment file:
   ```bash
   cp .env.example .env
   ```
3. Edit the `.env` file with your configuration:

   - `VITE_FRONTEND_BASE_URL` - Client URL with / (e.g., http://localhost:5173/)
   - `VITE_API_URL` - Backend URL with /api (e.g., http://localhost:3000/api)
   - `VITE_ENVIRONMENT` - DEVELOPMENT or PRODUCTION

4. Install dependencies:

   ```bash
   npm install
   ```

5. Run in development mode:

   ```bash
   npm run dev
   ```

   Or build for production:

   ```bash
   npm run build
   ```

### Backend Setup

1. Navigate to the backend directory:

   ```bash
   cd backend/
   ```

2. Create an environment file:
   ```bash
   cp .env.example .env
   ```
3. Edit the `.env` file with your configuration (see `.env.example` for details)

4. Install dependencies:

   ```bash
   go mod tidy
   ```

5. Ensure MongoDB and Redis are running

6. Start the backend:
   ```bash
   go run cmd/main.go
   ```

## Docker Compose Setup

NeoBase provides several Docker Compose configurations for different deployment scenarios:

### Local Development Setup

This setup includes everything you need to run NeoBase locally:

1. Navigate to the docker-compose directory:

   ```bash
   cd docker-compose/
   ```

2. Create an environment file in docker-compose folder:

   ```bash
   cp .env.example .env
   ```

3. Edit the `.env` file with your configuration

4. Create the network (first time only):

   ```bash
   docker network create neobase-network
   ```

5. Start the complete stack:
   ```bash
   docker-compose -f docker-compose-local.yml up -d --build
   ```

This will start:

- MongoDB and Redis (dependencies) (Update the Volume path for both)
- NeoBase backend
- NeoBase frontend

Access the application at http://localhost:5173

#### Running Example Databases (Optional)

To test with example databases:

```bash
docker-compose -f docker-compose-exampledbs.yml up -d
```

This will start:

- PostgreSQL (port 5432)
- ClickHouse (ports 8123, 9000)
- MySQL (port 3306)
- MongoDB (port 27017)

Access the application at your client hosted domain a.k.a `VITE_FRONTEND_BASE_URL`

### Server Deployment: Docker Compose

For production deployment on a server:

1. Navigate to the docker-compose directory:

   ```bash
   cd docker-compose/
   ```

2. Create an environment file in docker-compose folder:

   ```bash
   cp .env.example .env
   ```

3. Edit the `.env` file with your production configuration:

   - Set `ENVIRONMENT=PRODUCTION` and `VITE_ENVIRONMENT=PRODUCTION`
   - Configure your front end hosted url/domain in `CORS_ALLOWED_ORIGIN` and `VITE_FRONTEND_BASE_URL`
   - Set secure passwords for MongoDB and Redis
   - Add your OpenAI or Gemini API key

4. Create the network (first time only):

   ```bash
   docker network create neobase-network
   ```

5. Start the dependencies (if you don't have existing MongoDB/Redis):

   ```bash
   docker-compose -f docker-compose-dependencies.yml up -d
   ```

   ### Note: Update & mount the volume paths for the dependencies in above docker compose file.

6. Start the NeoBase applications:
   ```bash
   docker-compose -f docker-compose-server.yml up -d --build
   ```

### Server Deployment: Dokploy

NeoBase supports easy deployment to a server using [Dokploy](https://dokploy.com/). You must have your instance of Dokploy up and running for this.

Just follow these steps:

1. Create a new "Compose" service.

1. Select "Git" as a provider and enter the following data:

   - Repository URL: `https://github.com/bhaskarblur/neobase-ai-dba.git`
   - Branch: `main`
   - Compose Path: `./docker-compose/docker-compose-dokploy.yml`

1. Switch to the "Environment" tab and paste contents of the [`.env.example`](https://github.com/bhaskarblur/neobase-ai-dba/blob/main/docker-compose/.env.example) file and update with your production configration:

   - Set `ENVIRONMENT=PRODUCTION` and `VITE_ENVIRONMENT=PRODUCTION`
   - Configure your front end hosted url/domain in `CORS_ALLOWED_ORIGIN` and `VITE_FRONTEND_BASE_URL`
   - Set secure passwords for MongoDB and Redis
   - Add your OpenAI or Gemini API key

1. Switch to the "Domains" tab and add two domains. E.g. to use the same host for backend and client:

   - yourdomain.com for the `neobase-backend` service with path `/api` and port `3000`
   - yourdomain.com for the `neobase-client` service with path `/` and port `5173`

1. Start the NeoBase applications by clicking on [Deploy] button on the "General" tab.

## First-Time Setup

After deployment, follow these steps:

1. Access the client app at the configured URL (default: http://localhost:5173)

2. Generate a signup secret using admin credentials:

   - Send a POST request to your backend at`${BACKEND_URL}/api/auth/generate-signup-secret` with admin username & password(mentioned in your .env)
   - Example using curl:
     ```bash
     curl -X POST http://localhost:3000/api/auth/generate-signup-secret \
       -H "Content-Type: application/json" \
       -d '{"username":"your-admin-username","password":"your-admin-password"}'
     ```

3. Use the generated secret to create a new user through the NeoBase Client UI

4. Add database connections through the UI and start using NeoBase!

## Troubleshooting

- If containers fail to start, check logs with `docker-compose logs`
- Ensure all required ports are available (3000, 5173, 27017, 6379)
- Ensure you use hosted url with domains for both Backend and Client in environment variables if hosting on a server
- Verify your API keys are valid for OpenAI or Gemini

## Thank you for using NeoBase!

For more information, visit [neobase.cloud](https://neobase.cloud) or check our [GitHub repository](https://github.com/bhaskarblur/neobase-ai-dba).
