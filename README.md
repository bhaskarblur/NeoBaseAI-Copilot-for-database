
<!-- <img width="1707" alt="Screenshot 2025-02-10 at 7 12 19 PM" src="https://github.com/user-attachments/assets/50c36c8b-52f4-49c8-8b2b-d7b98279aa52" /> -->

<img width="1850" height="971" alt="neobase-banner" src="https://github.com/user-attachments/assets/1c88e3d2-6ae8-4398-a816-c5bf7224d08a" />

# NeoBase - AI Copilot for Database

**NeoBase** is an AI database copilot that allows you to chat with your data and visualize, analyze it quickly and easily. With a sleek Neo Brutalism design and real-time conversation, NeoBase makes database visualization intuitive and efficient.

## Screenshots

<!-- <img width="1708" alt="Screenshot 2025-03-14 at 1 08 09 PM" src="https://github.com/user-attachments/assets/413a2a91-98a3-4bda-b12f-46fb8c826f4a" /> -->
<img width="1709" height="901" alt="Screenshot 2025-12-17 at 12 42 17 AM" src="https://github.com/user-attachments/assets/b0d85b73-cdf4-4259-ad15-1b2a7d966ea0" />
<!-- <img width="1697" alt="Screenshot 2025-04-26 at 3 44 23 PM" src="https://github.com/user-attachments/assets/42828bb6-7725-4e5c-831a-8e4dc3990b19" /> -->
<img width="1709" height="900" alt="Screenshot 2025-12-17 at 12 44 37 AM" src="https://github.com/user-attachments/assets/732e96e9-7e30-433d-8ecd-5dd70474ce76" />


## Features

- **AI-Powered Queries**: Generate and optimize SQL queries using natural language prompts.
- **Multi-Database Support**: Connect to PostgreSQL, MySQL, MongoDB, Redis, and more.
- **Spreadsheet Support**: Upload and query CSV/Excel files with AES-GCM encryption.
- **Google Sheets Integration**: Connect directly to Google Sheets with OAuth authentication for seamless data access.
- **Real-Time Chat Interface**: Interact with your database like you're chatting with an expert.
- **Neo Brutalism Design**: Bold, modern, and high-contrast UI for a unique user experience.
- **Transaction Management**: Start, commit, and rollback transactions with ease.
- **Query Optimization**: Get AI-driven suggestions to improve query performance.
- **Schema Management**: Create indexes, views, and manage schemas effortlessly.
- **Self-Hosted & Open Source**: Deploy on your infrastructure with full control.

## Supported DBs
- PostgreSQL
- Yugabyte
- MySQL
- ClickHouse
- MongoDB
- Spreadsheet (CSV/Excel)
- Google Sheets (OAuth integration)

## Planned to be supported DBs
- Cassandra (Priority 1)
- Redis (Priority 2)
- Neo4j DB (Priority 3)

## Supported LLM Clients
- **OpenAI** (14 models) - GPT-5.2, O3, GPT-4.1, GPT-4o, and more cutting-edge models
- **Google Gemini** (7 models) - Gemini 3 Pro, Gemini 2.5 Flash/Pro, Gemini 2.0, and more
- **Anthropic Claude** (10 models) - Claude Opus 4.5 (world's best), Sonnet 4.5, Sonnet 4 (default), Haiku 4.5, 3.5 series, and 3.0 series
- **Ollama** (32+ models) - Self-hosted open-source models including DeepSeek R1, Llama 3.1/3.3, Qwen 2.5/3, Mistral, and more

### Dynamic Model Selection
Select different AI models for each message without restarting. Choose from **63 pre-configured models** across 4 providers with automatic API key filtering and intelligent routing.

### Configurable AI Providers
Enable or disable AI providers through simple environment variables:
- Set `OPENAI_API_KEY` → Enables all 14 OpenAI models
- Set `GEMINI_API_KEY` → Enables all 7 Gemini models
- Set `CLAUDE_API_KEY` → Enables all 10 Claude models
- Set `OLLAMA_BASE_URL` → Enables all 32+ Ollama models

No code changes needed - just configure your `.env` file. Use one provider or mix multiple providers based on your needs. You can also disable individual models at the code level by editing `IsEnabled` in `supported_models.go`. See [SETUP.md](SETUP.md) for detailed configuration examples.

### Enterprise & Self-Hosted Options
- Use Claude with your enterprise Anthropic license for world-class coding and reasoning
- Self-host models with Ollama for complete data privacy and cost control
- Mix cloud and self-hosted models based on your security and performance needs

## Tech Stack

- **Frontend**: React, Tailwind CSS
- **Backend**: Go (Gin framework)
- **App Used Database**: MongoDB, Redis
- **AI Orchestrator**: OpenAI, Google Gemini
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

## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.


Let me know if you'd like to add or modify anything!
