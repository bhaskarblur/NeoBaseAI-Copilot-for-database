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

- **OpenAI** (14 models) - GPT-5.2, O3, GPT-4.1, GPT-4o, and legacy models
- **Google Gemini** (7 models) - Gemini 3 Pro, Gemini 2.5 Flash/Pro, Gemini 2.0, and more
- **Anthropic Claude** (10 models) - Opus 4.5 (world's best), Sonnet 4.5, Sonnet 4 (default), Haiku 4.5, 3.5 series, and 3.0 series
- **Ollama** (32+ models) - Self-hosted open-source: DeepSeek R1, Llama 3.1/3.3, Qwen 2.5/3, Mistral, Gemma, Phi, and more

#### Dynamic Model Selection

NeoBase supports **dynamic LLM model selection** - you can choose a different AI model for each message without restarting the application. The system automatically:
- Filters available models based on configured API keys
- Displays model capabilities (token limits, descriptions)
- Allows per-message model selection via dropdown
- Persists model choice in chat history
- Routes each model to its correct provider (OpenAI, Gemini, Claude, or Ollama)

#### Enabling/Disabling AI Providers

NeoBase uses a **simple configuration-based approach** to enable or disable AI providers. Each provider is controlled by a single environment variable:

**How it works:**
1. **Set the environment variable** = Provider and all its models are enabled
2. **Leave it empty or unset** = Provider and all its models are disabled
3. **No code changes needed** - Just update your `.env` file

**Environment Variables:**
- `OPENAI_API_KEY` - Enables all 14 OpenAI models (GPT-5.2, O3, GPT-4o, etc.)
- `GEMINI_API_KEY` - Enables all 7 Gemini models (Gemini 3 Pro, 2.5 Flash, etc.)
- `CLAUDE_API_KEY` - Enables all 10 Claude models (Opus 4.5, Sonnet 4, etc.)
- `OLLAMA_BASE_URL` - Enables all 32+ Ollama models (default: http://localhost:11434)
- `DEFAULT_LLM_MODEL` - Default model ID (optional, auto-selects if not set)

**Requirements:**
- At least one API key/URL is required for the application to function
- You can enable multiple providers simultaneously (mix cloud and self-hosted)
- Models appear in the UI only when their provider is configured

**Examples:**
```bash
# Enable only OpenAI
OPENAI_API_KEY=sk-proj-abc123...

# Enable OpenAI + Claude (hybrid cloud)
OPENAI_API_KEY=sk-proj-abc123...
CLAUDE_API_KEY=sk-ant-xyz789...

# Enable Ollama only (self-hosted, privacy-focused)
OLLAMA_BASE_URL=http://localhost:11434

# Enable all providers (maximum flexibility)
OPENAI_API_KEY=sk-proj-abc123...
GEMINI_API_KEY=AIza...
CLAUDE_API_KEY=sk-ant-xyz789...
OLLAMA_BASE_URL=http://localhost:11434
```

#### Disabling Individual Models (Code Level)

If you want to disable specific models while keeping their provider enabled, edit `/backend/internal/constants/supported_models.go`:

**How to disable a model:**
1. Open `backend/internal/constants/supported_models.go`
2. Find the model in the `SupportedLLMModels` array
3. Change `IsEnabled: true` to `IsEnabled: false`
4. Restart the backend

**Example - Disable GPT-4o:**
```go
{
    ID:                  "gpt-4o",
    Provider:            OpenAI,
    DisplayName:         "GPT-4o (Omni)",
    IsEnabled:           false,  // Changed from true to false
    MaxCompletionTokens: 16384,
    Temperature:         1,
    InputTokenLimit:     128000,
    Description:         "Multimodal model for text and vision tasks",
}
```

**Example - Disable all Ollama vision models:**
```go
// Disable LLaVA
{
    ID:                  "llava:latest",
    IsEnabled:           false,  // Disabled
    // ... rest of config
}

// Disable Llama 3.2 Vision
{
    ID:                  "llama3.2-vision:11b",
    IsEnabled:           false,  // Disabled
    // ... rest of config
}
```

**When to use this:**
- Restrict expensive models in production
- Remove legacy/deprecated models
- Limit model selection for specific use cases
- Reduce UI clutter by hiding unused models

**Note:** This requires code changes and backend restart. For enabling/disabling entire providers, use environment variables instead.

**Enterprise & Self-Hosted Options:**
- Use Claude Opus 4.5 - the world's best model for coding, agents, and computer use
- Claude with your enterprise Anthropic license for advanced coding and reasoning
- Self-host models with Ollama for complete data privacy and cost control
- Mix cloud and self-hosted models based on your security and performance needs
- Choose from 63 total models across 4 providers for maximum flexibility

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
   - `VITE_GOOGLE_CLIENT_ID` - Google OAuth client ID (see [Creating Google OAuth Credentials](#creating-google-oauth-credentials))
   - `VITE_GOOGLE_REDIRECT_URI` - Google OAuth redirect URI (must match Google Cloud Console, e.g., http://localhost:5173/auth/google/callback)

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

   **Important LLM Configuration:**
   - `OPENAI_API_KEY` - Your OpenAI API key (enables 15 models)
   - `GEMINI_API_KEY` - Your Google Gemini API key (enables 7 models)
   - `DEFAULT_LLM_MODEL` - Default model ID (optional, automatically selects based on available keys)
   - At least one API key is required for the application to function

   **Important Spreadsheet & Google Sheets Configuration:**
   - `SPREADSHEET_POSTGRES_HOST` - PostgreSQL host for spreadsheet data storage
   - `SPREADSHEET_POSTGRES_PORT` - PostgreSQL port (default: 5432)
   - `SPREADSHEET_POSTGRES_DATABASE` - Database name for spreadsheet storage
   - `SPREADSHEET_POSTGRES_USERNAME` - PostgreSQL username
   - `SPREADSHEET_POSTGRES_PASSWORD` - PostgreSQL password
   - `SPREADSHEET_POSTGRES_SSL_MODE` - SSL mode (disable, require, verify-ca, verify-full)
   - `SPREADSHEET_DATA_ENCRYPTION_KEY` - 32-byte key for AES-GCM encryption of spreadsheet data

   **Important Google OAuth Configuration (used for both authentication and Google Sheets integration):**
   - `GOOGLE_CLIENT_ID` - Google OAuth client ID (see [Creating Google OAuth Credentials](#creating-google-oauth-credentials))
   - `GOOGLE_CLIENT_SECRET` - Google OAuth client secret (see [Creating Google OAuth Credentials](#creating-google-oauth-credentials))
   - `GOOGLE_REDIRECT_URL` - Google OAuth redirect URL (must match Google Cloud Console, e.g., http://localhost:5173/auth/google/callback)

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

## Google OAuth Authentication Setup

NeoBase supports Google OAuth2 authentication for user login and signup. This uses the same Google OAuth credentials configured for Google Sheets integration.

### Prerequisites

You need Google OAuth credentials set up. Follow the [Creating Google OAuth Credentials](#creating-google-oauth-credentials) section in the Google Sheets Integration Setup above to obtain your credentials.

### Frontend Configuration

Add these environment variables to your client `.env` file:

```bash
# Google OAuth Configuration for Authentication
VITE_GOOGLE_CLIENT_ID=your-client-id.apps.googleusercontent.com
VITE_GOOGLE_REDIRECT_URI=http://localhost:5173/auth/google/callback
```

**Important Notes**:
- `VITE_GOOGLE_CLIENT_ID` must match the client ID from Google Cloud Console
- `VITE_GOOGLE_REDIRECT_URI` must exactly match the redirect URI configured in Google Cloud Console
- Update these to your production domain when deploying

### Backend Configuration

The backend uses the same Google OAuth credentials as Google Sheets (see [Google Sheets Integration Setup](#google-sheets-integration-setup)):

```bash
GOOGLE_CLIENT_ID=your-client-id.googleusercontent.com
GOOGLE_CLIENT_SECRET=your-client-secret
GOOGLE_REDIRECT_URL=http://localhost:5173/auth/google/callback
```

### Authentication Flow

**Signup with Google** (in Production):
1. User clicks "Continue with Google" on signup form
2. System shows signup secret input field
3. User enters valid signup secret and validates it
4. User is redirected to Google OAuth consent screen
5. After authorization, account is created with Google credentials
6. User receives welcome email and is logged in

**Signup with Google** (in Development):
1. User clicks "Continue with Google" on signup form
2. User is immediately redirected to Google OAuth consent screen (no secret required)
3. After authorization, account is created
4. User receives welcome email and is logged in

**Login with Google**:
1. User clicks "Continue with Google" on login form
2. User is redirected to Google OAuth consent screen
3. After authorization, user is logged in to their existing account
4. If email matches an email-password account, error is shown (cannot mix auth types)

### Key Features

- **Dual-Purpose OAuth**: Same Google credentials for authentication and Sheets integration
- **Signup Secret Validation**: Production requires valid signup secret for new Google users (for security)
- **Email Conflict Prevention**: Email-password users cannot login with Google (proper error message)
- **Automatic Username Generation**: Creates unique usernames from Google profile data
- **JWT Session Management**: Same token system as email-password authentication
- **Welcome Email**: Sent automatically on successful signup
- **Development Mode**: No signup secret required in development environment

### Troubleshooting

**"Invalid signup secret" error**:
- Ensure you're using a valid signup secret generated by the admin
- In production, secrets are validated against the backend
- In development mode, signup secret is not required

**"This email is already associated with email-password auth" error**:
- The email you're trying to use with Google OAuth is already registered with password authentication
- Use a different email or login with your password instead
- This is intentional for security - you cannot mix authentication types

**"Redirect URI mismatch" error from Google**:
- Verify `VITE_GOOGLE_REDIRECT_URI` matches exactly what's configured in Google Cloud Console
- Include the protocol (http:// or https://)
- Update both frontend and Google Cloud Console for production domains

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
   - **Add at least one LLM API key:**
     - `OPENAI_API_KEY` - For OpenAI models (GPT-5.2, O3, GPT-4o, etc.)
     - `GEMINI_API_KEY` - For Google Gemini models (Gemini 3 Pro, 2.5 Flash, etc.)
   - Optionally set `DEFAULT_LLM_MODEL` to your preferred default model ID
   - **Configure Google OAuth credentials** (for both authentication and Google Sheets integration):
     - `GOOGLE_CLIENT_ID` - Google OAuth client ID (see [Creating Google OAuth Credentials](#creating-google-oauth-credentials))
     - `GOOGLE_CLIENT_SECRET` - Google OAuth client secret (see [Creating Google OAuth Credentials](#creating-google-oauth-credentials))
     - `GOOGLE_REDIRECT_URL` - Google OAuth redirect URL (must be your production domain, e.g., https://yourdomain.com/auth/google/callback)
     - `VITE_GOOGLE_CLIENT_ID` - Frontend Google OAuth client ID (same as backend GOOGLE_CLIENT_ID)
     - `VITE_GOOGLE_REDIRECT_URI` - Frontend Google OAuth redirect URI (must match GOOGLE_REDIRECT_URL)

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
   - **Add at least one LLM API key:**
     - `OPENAI_API_KEY` - For OpenAI models (GPT-5.2, O3, GPT-4o, etc.)
     - `GEMINI_API_KEY` - For Google Gemini models (Gemini 3 Pro, 2.5 Flash, etc.)
   - Optionally set `DEFAULT_LLM_MODEL` to your preferred default model ID
   - **Configure Google OAuth credentials** (for both authentication and Google Sheets integration):
     - `GOOGLE_CLIENT_ID` - Google OAuth client ID (see [Creating Google OAuth Credentials](#creating-google-oauth-credentials))
     - `GOOGLE_CLIENT_SECRET` - Google OAuth client secret (see [Creating Google OAuth Credentials](#creating-google-oauth-credentials))
     - `GOOGLE_REDIRECT_URL` - Google OAuth redirect URL (must be your production domain, e.g., https://yourdomain.com/auth/google/callback)
     - `VITE_GOOGLE_CLIENT_ID` - Frontend Google OAuth client ID (same as backend GOOGLE_CLIENT_ID)
     - `VITE_GOOGLE_REDIRECT_URI` - Frontend Google OAuth redirect URI (must match GOOGLE_REDIRECT_URL)

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
