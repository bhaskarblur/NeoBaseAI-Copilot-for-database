
<!-- <img width="1707" alt="Screenshot 2025-02-10 at 7 12 19 PM" src="https://github.com/user-attachments/assets/50c36c8b-52f4-49c8-8b2b-d7b98279aa52" /> -->

<img width="1850" height="971" alt="neobase-banner" src="https://github.com/user-attachments/assets/1c88e3d2-6ae8-4398-a816-c5bf7224d08a" />

<div align="center">

[![Discord](https://img.shields.io/badge/Discord-Join%20Community-5865F2?style=for-the-badge&logo=discord&logoColor=white)](https://discord.gg/VT9NRub86D)
[![Product Hunt](https://img.shields.io/badge/Product%20Hunt-Featured-DA552F?style=for-the-badge&logo=producthunt&logoColor=white)](https://www.producthunt.com/posts/neobase-2)
[![Website](https://img.shields.io/badge/Website-neobase.ai-FFDB58?style=for-the-badge&logo=google-chrome&logoColor=black)](https://neobase.cloud)
[![GitHub Stars](https://img.shields.io/github/stars/bhaskarblur/neobase-ai-dba?style=for-the-badge&logo=github&color=yellow)](https://github.com/bhaskarblur/neobase-ai-dba)
[![License](https://img.shields.io/badge/License-MIT-blue?style=for-the-badge)](LICENSE.md)

</div>

# NeoBase - AI Copilot for Database

**NeoBase** is an AI database copilot that allows you to chat with your data and visualize, analyze it quickly and easily. With a sleek Neo Brutalism design and real-time conversation, NeoBase makes database visualization intuitive and efficient.

## Screenshots
<img width="1709" height="900" alt="create connection" src="https://github.com/user-attachments/assets/732e96e9-7e30-433d-8ecd-5dd70474ce76" />
<img width="1709" height="901" alt="chat interface" src="https://github.com/user-attachments/assets/b0d85b73-cdf4-4259-ad15-1b2a7d966ea0" />
<img width="1709" height="901" alt="dashboard screenshot" src="https://github.com/user-attachments/assets/c2da02f0-2dec-4561-99d1-97d820d43f76" />


## Features

- **AI-Powered Queries with Agentic Tool Calling**: Generate and optimize SQL & NoSQL queries using natural language prompts through an intelligent agentic system. The AI autonomously executes queries, analyzes results, and iteratively refines its approach until delivering the perfect answer—all without manual intervention.
- **RAG-Powered Context Awareness**: Hybrid RAG (Retrieval-Augmented Generation) system using Qdrant vector search for intelligent context retrieval. Schema RAG finds relevant tables/collections from your database structure, while Message RAG retrieves semantically similar past conversations—ensuring the AI always has the right context.
- **AI-Generated Dashboards**: Build comprehensive data dashboards with natural language prompts. The AI analyzes your database schema and generates intelligent dashboard blueprints with 11 widget types—**stat cards** for KPIs, **data tables** for detailed records, **interactive charts** (line, bar, pie, area), **gauge widgets** (speedometer-style indicators with color thresholds), **bar gauge widgets** (horizontal progress bars with multiple display modes), **heatmap widgets** (2D grid visualizations for pattern analysis), and **histogram widgets** (distribution charts with statistical markers). Features include dashboard import/export (JSON), regeneration with variants, real-time data refresh, and seamless duplication across chats.
- **Multi-Database Support**: Connect to PostgreSQL, MySQL, MongoDB, Redis, and more.
- **Spreadsheet Support**: Upload and query CSV/Excel files with AES-GCM encryption.
- **Google Sheets Integration**: Connect directly to Google Sheets with OAuth authentication for seamless data access.
- **AI-Generated Visualizations**: Automatically generate meaningful charts from query results. The AI analyzes query output and creates appropriate visualizations (line, bar, pie, area, scatter charts) with smart data mapping and aggregation.
- **Real-Time Chat Interface**: Interact with your database like you're chatting with an expert.
- **Neo Brutalism Design**: Bold, modern, and high-contrast UI for a unique user experience.
- **Transaction Management**: Start, commit, and rollback transactions with ease.
- **Query Optimization**: Get AI-driven suggestions to improve query performance.
- **Schema Management**: Create indexes, views, and manage schemas effortlessly.
- **Self-Hosted & Open Source**: Deploy on your infrastructure with full control.
- **Authentication & Signup Secret**: Restrict access via signup secret & use email-password, google signup

## Supported DBs
- PostgreSQL
- Timescale
- Yugabyte
- MySQL
- Starrocks
- ClickHouse
- MongoDB
- Spreadsheet (CSV/Excel)
- Google Sheets (OAuth integration)

## Visualization Architecture

**NeoBase Visualization** is an AI-powered feature that transforms query results into meaningful charts. Here's how it works:

### Query-First Design
- **Visualization is a child of Query**: Each query produces result rows, and only queries with results can generate visualizations
- **1:1 Relationship**: Each query can have its own independent visualization, enabling multiple charts in a single message
- **Persistent State**: Visualization configurations are stored in the database, preventing regeneration of the same query's visualization

### Smart Chart Generation
The AI analyzes your query results and automatically:
- **Detects data types** (dates, numbers, categories, text)
- **Selects optimal chart type**:
  - **Line Chart**: Time series data and trends over time
  - **Bar Chart**: Categorical comparisons and rankings
  - **Pie Chart**: Proportions and percentages
  - **Area Chart**: Cumulative metrics and stacked data
  - **Scatter Plot**: Correlations between two numeric values
- **Maps columns intelligently** (dates → X-axis, metrics → Y-axis, categories → labels)
- **Aggregates large datasets** (100k+ rows) with smart sampling and grouping strategies

### Supported Database-Specific Analysis
Each database type has specialized prompt logic for:
- **PostgreSQL/Yugabyte**: Advanced date/time handling, aggregate functions, complex types
- **MySQL**: DATE/DATETIME/TIMESTAMP types, INT/DECIMAL precision
- **MongoDB**: ISODate handling, aggregation pipelines, document structure
- **ClickHouse**: DateTime64 optimization, Array types, time-series analysis
- **Spreadsheets**: CSV/Excel data type detection and conversion

## Planned to be supported DBs
- Cassandra (Priority 1)
- Redis (Priority 2)
- Neo4j DB (Priority 3)

## Supported LLM Clients
- **OpenAI** (18 models) - GPT-5.2, O3/O4, GPT-4.1, GPT-4o, and more cutting-edge models
- **Google Gemini** (9 models) - **Gemini 3 Pro/Flash** (latest), Gemini 2.5 Pro/Flash, and more
- **Anthropic Claude** (12 models) - **Claude Opus 4.6** (most intelligent), Sonnet 4.6/4.5, Sonnet 4 (default), and more
- **Ollama** (30+ models) - Self-hosted open-source models including DeepSeek R1, Llama 3.1/3.3, Qwen 2.5/3, Mistral, and more

### Dynamic Model Selection
Select different AI models for each message without restarting. Choose from **65+ pre-configured models** across 4 providers with automatic API key filtering and intelligent routing.

### Configurable AI Providers
Enable or disable AI providers through simple environment variables:
- Set `OPENAI_API_KEY` → Enables all 14 OpenAI models
- Set `GEMINI_API_KEY` → Enables all 7 Gemini models
- Set `CLAUDE_API_KEY` → Enables all 10 Claude models
- Set `OLLAMA_BASE_URL` → Enables all 30+ Ollama models

No code changes needed - just configure your `.env` file. Use one provider or mix multiple providers based on your needs. You can also disable individual models at the code level by editing `IsEnabled` in `supported_models.go`. See [SETUP.md](SETUP.md) for detailed configuration examples.

### Enterprise & Self-Hosted Options
- Use Claude with your enterprise Anthropic license for world-class coding and reasoning
- Self-host models with Ollama for complete data privacy and cost control
- Mix cloud and self-hosted models based on your security and performance needs

## Tech Stack

- **Frontend**: React, Tailwind CSS
- **Backend**: Go (Gin framework)
- **App Used Database**: MongoDB, Redis
- **AI Orchestrator**: OpenAI, Google Gemini, Claude, Ollama
- **Database Drivers**: PostgreSQL, Yugabyte, MySQL, MongoDB, Redis, Neo4j, etc.
- **Styling**: Neo Brutalism design with custom Tailwind utilities


## Getting Started

## How to setup
Read ([SETUP](https://github.com/bhaskarblur/neobase-ai-dba/blob/main/SETUP.md)) to learn how to setup NeoBase on your system.
## Usage

1. **Create a new user in the app**:
   - Open the client app on `http://localhost:5173` in your browser.
   - Admin credentials are set via `ADMIN_USERNAME` and `ADMIN_PASSWORD` environment variables.
   - Creating a new user requires an username, password and user signup secret.
   - User signup secret is generated via Admin credenials by sending a POST request to `api/auth/generate-signup-secret` with admin username & password in the body
   - Use this secret to signup a new user from NeoBase UI.

2. **Add a Database Connection**:
   - Click "Add Connection" in the sidebar.
   - Choose & Enter your database credentials (e.g., host, port, username, password).
   - Click "Save" to add the connection.

3. **Chat with Your Database**:
   - Type natural language prompts in the chat interface (e.g., "Show me all users").
   - View the generated SQL, Other DBs query and results.
   - Paginated results that support large volume of data.

4. **Manage Transactions**:
   - Run the query in transaction mode by clicking "Play" icon button in query.
   - You can also cancel the query by clicking "Cancel" icon button in query.
   - Perform rollbacks by clicking "History" icon button in query.

5. **Optimize Queries**:
   - Get AI-driven suggestions to improve query performance.

6. **Visualize Query Results**:
   - AI automatically analyzes query output and generates meaningful charts.
   - Each query result can have its own visualization (line, bar, pie, area, scatter).
   - Enable "Auto Generate Visualizations" in chat settings for automatic chart creation.
   - Manually generate visualizations for any query result with the "Generate Visualization" button.
   - Visualizations are intelligently mapped to your data (dates on X-axis, metrics on Y-axis, etc.).
   - Supports large datasets with smart aggregation and sampling strategies.


## Community

Join our [Discord community](https://discord.gg/VT9NRub86D) to connect with other users, share feedback, discuss features, and get help from the NeoBase team!

## Contributing

We welcome contributions! Here's how you can help:

1. Fork the repository.
2. Create a new branch (`git checkout -b feature/your-feature`).
3. Commit your changes (`git commit -m 'Add some feature'`).
4. Push to the branch (`git push origin feature/your-feature`).
5. Open a pull request.

See the list of contributors in [CONTRIBUTORS](CONTRIBUTORS) file.

### Contributor Guides

Detailed step-by-step guides for common contribution tasks:

- **[How to Add a New Database](docs/HOW-TO-IMPLEMENT-NEW-DB.md)** — Full walkthrough for adding a new DB type (backend driver, schema fetcher, frontend UI, Docker Compose example, landing page, and testing checklist).
- **[How to Add a New LLM Provider or Model](docs/HOW-TO-IMPLEMENT-NEW-LLM.md)** — Full walkthrough for adding a new LLM provider (client implementation, model catalogue, env vars, DI wiring) or adding a new model to an existing provider.

## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.
