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
- **PostgreSQL** (for Spreadsheet feature) - Stores uploaded CSV/Excel data with encryption

### Supported Databases (for querying)

- PostgreSQL
- MySQL
- Yugabyte
- ClickHouse
- MongoDB
- Spreadsheet (CSV/Excel) - Upload and query CSV/Excel files with AES-GCM encryption
- Google Sheets - Connect directly to Google Sheets with OAuth authentication
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

   **Important Spreadsheet & Google Sheets Configuration:**
   - `SPREADSHEET_POSTGRES_HOST` - PostgreSQL host for spreadsheet data storage
   - `SPREADSHEET_POSTGRES_PORT` - PostgreSQL port (default: 5432)
   - `SPREADSHEET_POSTGRES_DATABASE` - Database name for spreadsheet storage
   - `SPREADSHEET_POSTGRES_USERNAME` - PostgreSQL username
   - `SPREADSHEET_POSTGRES_PASSWORD` - PostgreSQL password
   - `SPREADSHEET_POSTGRES_SSL_MODE` - SSL mode (disable, require, verify-ca, verify-full)
   - `SPREADSHEET_DATA_ENCRYPTION_KEY` - 32-byte key for AES-GCM encryption of spreadsheet data
   - `GOOGLE_CLIENT_ID` - Google OAuth client ID for Google Sheets integration
   - `GOOGLE_CLIENT_SECRET` - Google OAuth client secret for Google Sheets integration  
   - `GOOGLE_REDIRECT_URL` - Google OAuth redirect URL (e.g., http://localhost:5173/auth/google/callback)

4. Install dependencies:

   ```bash
   go mod tidy
   ```

5. Ensure MongoDB, Redis, and PostgreSQL (for spreadsheet feature) are running

6. Start the backend:
   ```bash
   go run cmd/main.go
   ```

## Google Sheets Integration Setup

To enable Google Sheets connectivity, you need to set up Google OAuth credentials:

### Creating Google OAuth Credentials

1. **Go to Google Cloud Console**:
   - Visit [Google Cloud Console](https://console.cloud.google.com/)
   - Sign in with your Google account

2. **Create or Select a Project**:
   - Create a new project or select an existing one
   - Note down your project ID

3. **Enable Required APIs**:
   - Navigate to "APIs & Services" > "Library"
   - Enable the following APIs:
     - **Google Sheets API** - Search and enable "Google Sheets API"
     - **Google+ API** (optional but recommended) - Search and enable "Google+ API" for better profile information

4. **Create OAuth 2.0 Credentials**:
   - Go to "APIs & Services" > "Credentials"
   - Click "Create Credentials" > "OAuth client ID"
   - If prompted, configure the OAuth consent screen first:
     - Choose "External" user type
     - Fill in required fields (App name, User support email, Developer contact)
     - **Add the following scopes in the "Scopes" section**:
       - `https://www.googleapis.com/auth/spreadsheets.readonly` - Required for reading Google Sheets data
       - `https://www.googleapis.com/auth/userinfo.email` - Required for user identification
     - Add test users if needed (add your Gmail address for testing)

5. **Configure OAuth Client ID**:
   - Choose "Web application" as the application type
   - Add authorized redirect URIs:
     - For local development: `http://localhost:5173/auth/google/callback`
     - For production: `https://yourdomain.com/auth/google/callback`
   - Click "Create"

6. **Get Your Credentials**:
   - Copy the **Client ID** (ends with `.googleusercontent.com`)
   - Copy the **Client Secret**
   - Save these securely

### Environment Configuration

Add these variables to your `.env` file:

```bash
# Google OAuth Configuration for Google Sheets Integration
GOOGLE_CLIENT_ID=your-client-id.googleusercontent.com
GOOGLE_CLIENT_SECRET=your-client-secret
GOOGLE_REDIRECT_URL=http://localhost:5173/auth/google/callback
```

**Important Notes**:
- Replace `your-client-id` and `your-client-secret` with actual values from Google Cloud Console
- Update `GOOGLE_REDIRECT_URL` to match your domain in production
- The redirect URL must exactly match what you configured in Google Cloud Console

### Testing Google Sheets Connection

1. Start NeoBase with the Google OAuth configuration
2. In the NeoBase UI, create a new connection
3. Select "Google Sheets" as the connection type
4. Click "Connect" to initiate OAuth flow
5. Authorize NeoBase to access your Google Sheets
6. Enter the Google Sheets URL you want to connect to

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
- PostgreSQL for spreadsheet data storage (port 5433)
- NeoBase backend
- NeoBase frontend

Access the application at http://localhost:5173

**Note:** The spreadsheet PostgreSQL database runs on port 5433 to avoid conflicts with any existing PostgreSQL installations.

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
   - Optionally configure `LANDING_PAGE_CORS_ALLOWED_ORIGIN` if you have a separate landing page domain
   - Set secure passwords for MongoDB and Redis
   - Add your OpenAI or Gemini API key
   - Configure Google OAuth credentials if you want Google Sheets integration (see [Google Sheets Setup](#google-sheets-integration-setup))

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
   - Optionally configure `LANDING_PAGE_CORS_ALLOWED_ORIGIN` if you have a separate landing page domain
   - Set secure passwords for MongoDB and Redis
   - Add your OpenAI or Gemini API key
   - Configure Google OAuth credentials if you want Google Sheets integration (see [Google Sheets Setup](#google-sheets-integration-setup))

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
