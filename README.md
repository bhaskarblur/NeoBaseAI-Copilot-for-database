# NeoBase - AI Copilot for Database
<img width="1707" alt="Screenshot 2025-02-10 at 7 12 19 PM" src="https://github.com/user-attachments/assets/50c36c8b-52f4-49c8-8b2b-d7b98279aa52" />


**NeoBase** is an AI-powered database assistant that helps you manage, query, and optimize your databases effortlessly. With a sleek Neo Brutalism design and real-time chat functionality, NeoBase makes database visualization intuitive and efficient.

## Screenshots
<img width="1708" alt="Screenshot 2025-03-14 at 1 08 09 PM" src="https://github.com/user-attachments/assets/413a2a91-98a3-4bda-b12f-46fb8c826f4a" />
<img width="1697" alt="Screenshot 2025-04-26 at 3 44 23 PM" src="https://github.com/user-attachments/assets/42828bb6-7725-4e5c-831a-8e4dc3990b19" />


## Features

- **AI-Powered Queries**: Generate and optimize SQL queries using natural language prompts.
- **Multi-Database Support**: Connect to PostgreSQL, MySQL, MongoDB, Redis, and more.
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

## Planned to be supported DBs
- Cassandra (Priority 1)
- Redis (Priority 2)
- Neo4j DB (Priority 3)

## Supported LLM Clients
- OpenAI (Any chat completion model)
- Google Gemini (Any chat completion model)
- Ollama (Any chat completion model)

## Planned to be supported LLM Clients
- Anthropic (Claude 3.5 Sonnet)

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
