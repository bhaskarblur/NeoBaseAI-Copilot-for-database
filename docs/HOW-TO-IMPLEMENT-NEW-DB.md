# How to Add a New Database to NeoBase

This document is the single source of truth for contributors adding support for a new database
engine. Work through it top-to-bottom; each section tells you **what file to change, exactly what
to write, and — critically — why it matters**. Skip a step and the DB will silently misbehave.

The guide is split into four layers in order: **Backend → Frontend Client → Docker Compose →
Landing Page**, followed by a **Testing** section.

---

## Real-world examples in this codebase

| DB | Parent | Type | Key files |
|---|---|---|---|
| TimescaleDB | PostgreSQL | Wrapper | `constants/timescaledb.go` |
| StarRocks | MySQL | Wrapper | `constants/starrocks.go` |
| ClickHouse | — | Native | `pkg/dbmanager/clickhouse_driver.go` |
| MongoDB | — | Native | `pkg/dbmanager/mongodb_driver.go` |

---

## Terminology

| Term | Meaning |
|---|---|
| **Native DB** | A database with its own wire protocol and driver (e.g. ClickHouse, MongoDB). All driver, schema-fetcher, and wrapper code must be written from scratch. |
| **Wrapper DB** | A database that is wire-compatible with an existing driver (e.g. TimescaleDB ≅ PostgreSQL, StarRocks ≅ MySQL). Driver/wrapper code is reused; only prompt content and a few metadata lookups are customised. |
| **Prompt extension** | A small `const` string appended to a parent's full prompt, capturing only the *delta* behaviour of the wrapper DB. |
| **Schema fetcher** | The `SchemaFetcher` interface implementation that knows how to introspect table names, column types, indexes, foreign keys, etc. for a given DB. |
| **DBExecutor / Wrapper** | The `DBExecutor` interface implementation that all query paths go through. One wrapper exists per wire-protocol family (PostgresWrapper, MySQLWrapper, etc.). |

---

## Part 1 — Backend (`backend/`)

The backend handles connection management, query execution, schema introspection, and all LLM
orchestration. Most of the work is here.

---

### Step 1.1 — Register the DB type constant

**File:** `backend/internal/constants/databases.go`

```go
const (
    DatabaseTypePostgreSQL  = "postgresql"
    DatabaseTypeMySQL       = "mysql"
    DatabaseTypeYugabyteDB  = "yugabytedb"
    DatabaseTypeTimescaleDB = "timescaledb"
    DatabaseTypeStarRocks   = "starrocks"
    // ...
    DatabaseTypeMyNewDB     = "mynewdb"   // ← ADD THIS
)
```

**Why:** This lowercase string is the canonical identifier for the DB type used *everywhere* in the
system — switch statements, map keys, DTO validation, driver registration, and the
docker-compose-based example-db names all reference this constant. Defining it once in one place
eliminates typos across ~20 files.

> **Convention:** always lowercase, no spaces, matches the `type` value the frontend sends in JSON.

---

### Step 1.2 — Write the LLM prompt

The system prompt is the most important piece of the integration. It shapes every SQL query the
LLM generates for this database.

#### Option A — Native DB: create a new full prompt file

**File:** `backend/internal/constants/mynewdb.go` *(create new file)*

```go
package constants

// MyNewDBPrompt is the system prompt used for all MyNewDB connections.
// It is provider-agnostic — the same text is sent to OpenAI, Gemini, Claude, and Ollama.
const MyNewDBPrompt = `You are NeoBase AI, an expert MyNewDB database assistant.

Your primary role is to help users query and understand their MyNewDB databases.

## Core Rules
1. ONLY generate SELECT queries unless the user explicitly asks to modify data.
2. Always use proper MyNewDB syntax.
3. Never expose passwords, secrets, or sensitive configuration.
... (model on postgresql.go or mysql.go)
`

// MyNewDBVisualizationPrompt is used when the LLM must suggest a chart type.
// It describes MyNewDB's data patterns and typical visualizations.
const MyNewDBVisualizationPrompt = `
... (model on postgresql.go visualization prompt)
`
```

Include in the prompt:
- Full query syntax rules and dialect quirks specific to this DB
- Schema introspection idioms (e.g. how to list tables, describe columns)
- Things the LLM must **never** do (e.g. `DROP TABLE`, update compressed rows, etc.)
- Performance hints unique to this engine
- Sample query patterns for common analytical questions

#### Option B — Wrapper DB: create an extension file

If the new DB reuses a parent wire protocol, create a *small extension* file instead of a full
prompt. The parent prompt is prepended automatically (see Step 1.3).

**File:** `backend/internal/constants/mynewdb.go` *(create new file)*

```go
package constants

// MyNewDBExtensions is appended verbatim to the parent DB's full prompt.
// Captures only the delta behaviour — things that are different or additional in MyNewDB.
const MyNewDBExtensions = `

---
### MyNewDB-Specific Rules (these append to the parent rules above)

You are assisting a **MyNewDB** database. All parent rules apply. Additionally:

1. **Unique Function: foo_aggregate()**
   - Use foo_aggregate() instead of COUNT(DISTINCT ...) for better performance.

2. **Table Type Awareness**
   - MyNewDB has two table types: Distributed and Replicated.
   - Always filter on shard keys first to avoid cross-shard scans.

3. **Syntax Delta**
   - Use TODATE() not DATE(): WHERE TODATE(event_ts) = today()
`

// MyNewDBVisualizationExtensions is appended to the parent's visualization prompt.
const MyNewDBVisualizationExtensions = `

MyNewDB visualization guidance:
- Default to LINE charts for time-series data.
- Use STAT cards for KPI values.
`
```

**Why the extension approach:** The parent prompt is battle-tested. Bug-fixes to it automatically
benefit wrapper DBs. The extension file captures *only the delta* so reviewers immediately see
what is different about this DB. Extensions are cheap to maintain.

---

### Step 1.3 — Wire the prompt into the LLM dispatcher

**File:** `backend/internal/constants/llms.go`

Three functions dispatch prompts. Add a case to all three.

**1.3a — `getDatabasePrompt` (main chat prompt)**

```go
func getDatabasePrompt(dbType string) string {
    switch dbType {
    case DatabaseTypePostgreSQL:
        return PostgreSQLPrompt
    // ...
    case DatabaseTypeMyNewDB:
        return MyNewDBPrompt                        // Option A — native
        // OR for wrapper:
        // return ParentDBPrompt + MyNewDBExtensions
    default:
        return PostgreSQLPrompt
    }
}
```

**1.3b — `getNonTechModeInstructions` (non-technical user mode)**

Non-tech mode strips SQL jargon from assistant responses for business users. Group the new DB with
its closest parent if wire-compatible:

```go
func getNonTechModeInstructions(dbType string) string {
    // ...
    switch dbType {
    case DatabaseTypeMongoDB:
        return baseInstructions + getMongoDBNonTechInstructions()
    case DatabaseTypePostgreSQL, DatabaseTypeYugabyteDB, DatabaseTypeTimescaleDB, DatabaseTypeMyNewDB:
        return baseInstructions + getPostgreSQLNonTechInstructions()
    case DatabaseTypeMySQL, DatabaseTypeStarRocks:
        return baseInstructions + getMySQLNonTechInstructions()
    // OR for a truly distinct DB:
    case DatabaseTypeMyNewDB:
        return baseInstructions + getMyNewDBNonTechInstructions() // write this helper
    }
}
```

**1.3c — `GetVisualizationPrompt` (dashboard / chart generation)**

```go
func GetVisualizationPrompt(dbType string) string {
    switch dbType {
    // ...
    case DatabaseTypeMyNewDB:
        return MyNewDBVisualizationPrompt                       // native
        // OR for wrapper:
        // return ParentVisualizationPrompt + MyNewDBVisualizationExtensions
    }
}
```

**Why all three:** The chat dispatcher, the non-tech mode pre-processor, and the dashboard
visualization service each call a separate function. Missing any one of them causes the LLM to
fall back to the PostgreSQL prompt — it will still *work*, but it will generate wrong syntax for
your DB or miss database-specific functions entirely.

---

### Step 1.4 — Register the driver and schema fetcher

This is done in **two places** that must stay in sync.

#### Step 1.4a — DI container (production startup path)

**File:** `backend/internal/di/modules.go`

The DI container runs at application startup. Find the `dbmanager.NewManager` `Provide` block and
add your DB to both sections:

```go
// === Driver registration ===
// A "driver" here is a struct implementing DBExecutor — it knows how to open connections.

// For a native DB:
manager.RegisterDriver(constants.DatabaseTypeMyNewDB, dbmanager.NewMyNewDBDriver())

// For a Postgres-wire wrapper:
manager.RegisterDriver(constants.DatabaseTypeMyNewDB, dbmanager.NewPostgresDriver())

// For a MySQL-wire wrapper:
manager.RegisterDriver(constants.DatabaseTypeMyNewDB, dbmanager.NewMySQLDriver())


// === Schema fetcher registration ===
// A "fetcher" is a factory function that returns a SchemaFetcher for a live connection.
// The factory receives the DBExecutor so it can issue introspection queries.

// For a native DB with its own introspection:
manager.RegisterFetcher(constants.DatabaseTypeMyNewDB, func(db dbmanager.DBExecutor) dbmanager.SchemaFetcher {
    return &dbmanager.MyNewDBDriver{}
})

// For a Postgres-wire wrapper (shares PostgreSQL information_schema):
manager.RegisterFetcher(constants.DatabaseTypeMyNewDB, func(db dbmanager.DBExecutor) dbmanager.SchemaFetcher {
    return &dbmanager.PostgresDriver{} // reuses PostgreSQL's SchemaFetcher
})

// For a MySQL-wire wrapper (shares MySQL SHOW TABLES introspection):
manager.RegisterFetcher(constants.DatabaseTypeMyNewDB, func(db dbmanager.DBExecutor) dbmanager.SchemaFetcher {
    return dbmanager.NewMySQLSchemaFetcher(db) // MySQL fetcher needs the live connection
})
```

Also add a `LLMDBConfig` entry inside **each** of the four LLM provider blocks (OpenAI, Gemini,
Claude, Ollama). They all follow the same pattern — just change the provider constant:

```go
// Inside the OpenAI block (repeat pattern for Gemini, Claude, Ollama)
DBConfigs: []llm.LLMDBConfig{
    // ... existing entries ...
    {
        DBType:       constants.DatabaseTypeMyNewDB,
        Schema:       constants.GetLLMResponseSchema(constants.OpenAI, constants.DatabaseTypeMyNewDB),
        SystemPrompt: constants.GetSystemPrompt(constants.OpenAI, constants.DatabaseTypeMyNewDB, false),
    },
},
```

**Why:** `LLMDBConfig` pre-loads the system prompt and response schema at startup so the LLM
manager can do a fast lookup per request without re-computing. The four provider blocks each need
the entry because any of the four providers might be the active one.

#### Step 1.4b — Low-level manager fallback (non-DI path)

**File:** `backend/pkg/dbmanager/manager_crud.go`

The `registerDefaultDrivers` function mirrors the DI registration for contexts that do not use
the DI container (e.g. tests, manual manager construction):

```go
func (m *Manager) registerDefaultDrivers() {
    // ...
    // Register MyNewDB driver
    m.RegisterDriver("mynewdb", NewMyNewDBDriver())
    // OR reuse parent: m.RegisterDriver("mynewdb", NewPostgresDriver())

    // Register MyNewDB schema fetcher
    m.RegisterFetcher("mynewdb", func(db DBExecutor) SchemaFetcher {
        return &MyNewDBDriver{}
        // OR reuse parent: return &PostgresDriver{}
    })
}
```

Also update the `GetConnection` switch so connections of this type receive the right `DBExecutor`
wrapper:

```go
func (m *Manager) GetConnection(chatID string) (DBExecutor, error) {
    conn := m.connections[chatID]
    switch conn.Config.Type {
    case constants.DatabaseTypePostgreSQL, constants.DatabaseTypeYugabyteDB,
        constants.DatabaseTypeTimescaleDB, constants.DatabaseTypeMyNewDB: // ← add
        return conn.DB.(*PostgresWrapper), nil
    case constants.DatabaseTypeMySQL, constants.DatabaseTypeStarRocks:
        return conn.DB.(*MySQLWrapper), nil
    // ...
    }
}
```

**Why:** The `GetConnection` switch determines which `DBExecutor` implementation a service
receives. Getting this wrong means queries are routed to the wrong wrapper and may panic at
runtime on type assertions.

---

### Step 1.5 — Write the driver (native DBs only)

Skip this step for wrapper DBs — they reuse the parent driver.

**File:** `backend/pkg/dbmanager/mynewdb_driver.go` *(create new file)*

The driver must implement at minimum the `SchemaFetcher` interface:

```go
type MyNewDBDriver struct{}

// GetSchema returns a full snapshot of the database schema.
// selectedTables is a slice of table names to include (or ["ALL"] to include everything).
func (d *MyNewDBDriver) GetSchema(ctx context.Context, db DBExecutor, selectedTables []string) (*SchemaInfo, error) {
    // Issue introspection queries against the live db to populate SchemaInfo
    // Populate Tables, Indexes, ForeignKeys, Constraints
}

// GetTableChecksum returns a hash of a single table's structure for change detection.
func (d *MyNewDBDriver) GetTableChecksum(ctx context.Context, db DBExecutor, table string) (string, error) {
    // Fetch column definitions for `table`, produce a deterministic hash
}
```

If the DB uses its own wire protocol (not PG or MySQL), also create:

**File:** `backend/pkg/dbmanager/mynewdb_wrapper.go` *(create new file)*

The wrapper must implement `DBExecutor`:

```go
type MyNewDBWrapper struct {
    BaseWrapper
}

func NewMyNewDBWrapper(db *gorm.DB, manager *Manager, chatID string) *MyNewDBWrapper { ... }

func (w *MyNewDBWrapper) GetSchema(ctx context.Context) (*SchemaInfo, error)     { ... }
func (w *MyNewDBWrapper) GetTableChecksum(ctx context.Context, table string) (string, error) { ... }
func (w *MyNewDBWrapper) Raw(sql string, values ...interface{}) error            { ... }
func (w *MyNewDBWrapper) Exec(sql string, values ...interface{}) error           { ... }
func (w *MyNewDBWrapper) Query(sql string, dest interface{}, values ...interface{}) error { ... }
func (w *MyNewDBWrapper) QueryRows(sql string, dest *[]map[string]interface{}, values ...interface{}) error { ... }
func (w *MyNewDBWrapper) Close() error                                          { ... }
func (w *MyNewDBWrapper) GetDB() *sql.DB                                        { ... }
```

**Why:** The driver is the only component that understands the DB's metadata API (e.g.
`information_schema`, `SHOW TABLES`, `system.columns`, `db.getCollectionNames()`). Everything
above it works on the generic `SchemaInfo` struct. Get this right and schema display, vector
embeddings, and query suggestions all work for free.

---

### Step 1.6 — Add query validation

**File:** `backend/pkg/dbmanager/query_validator.go`

The query validator catches destructive queries (DROP TABLE without a flag, DELETE without WHERE,
etc.) before they are sent to the DB.

```go
func NewQueryValidator(dbType string) QueryValidator {
    switch dbType {
    case "postgresql", "yugabytedb", "timescaledb":
        return NewSQLQueryValidator("postgresql")
    case "mysql", "starrocks":
        return NewSQLQueryValidator("mysql")
    case "mongodb":
        return NewMongoQueryValidator()
    case "mynewdb":                              // ← ADD
        return NewSQLQueryValidator("postgresql") // or "mysql" — whichever dialect is closest
    default:
        return NewSQLQueryValidator("postgresql")
    }
}
```

**Why:** `NewSQLQueryValidator(dialect)` uses a dialect-specific lexer. Mapping to the closest
parent gives free protection against destructive queries without writing a bespoke parser.

---

### Step 1.7 — Add query classification

**File:** `backend/internal/constants/query_classification.go`

```go
var queryClassificationMap = map[string]QueryClassification{
    DatabaseTypePostgreSQL:   PostgreSQLQueryClassification,
    DatabaseTypeYugabyteDB:   YugabyteDBQueryClassification,
    DatabaseTypeTimescaleDB:  PostgreSQLQueryClassification, // TimescaleDB extends PostgreSQL
    DatabaseTypeMySQL:        MySQLQueryClassification,
    DatabaseTypeStarRocks:    MySQLQueryClassification,     // StarRocks is MySQL-wire-compatible
    DatabaseTypeClickhouse:   ClickHouseQueryClassification,
    DatabaseTypeMongoDB:      MongoDBQueryClassification,
    DatabaseTypeMyNewDB:      PostgreSQLQueryClassification, // ← ADD — use closest parent
    // ...
}
```

**Why:** `GetQueryClassification` determines whether a query is DDL, DML, or a plain read.
This controls schema-change triggers (DDL → invalidate schema cache), audit events, and the
`auto_execute_query` safety gate. A missing entry falls back to PostgreSQL classification, but
an explicit entry is more correct and more reviewable.

---

### Step 1.8 — Add cursor / pagination support

**File:** `backend/pkg/dbmanager/cursor_utils.go`

Cursor-based pagination injects a `{{cursor_value}}` placeholder into paginated queries called by
the frontend when scrolling through results.

```go
switch dbType {
case constants.DatabaseTypePostgreSQL, constants.DatabaseTypeMySQL,
    constants.DatabaseTypeYugabyteDB, constants.DatabaseTypeTimescaleDB,
    constants.DatabaseTypeStarRocks, constants.DatabaseTypeClickhouse,
    constants.DatabaseTypeMyNewDB: // ← ADD to this SQL case
    return strings.ReplaceAll(paginatedQuery, placeholder, sqlFormatCursorValue(cursorValue))
default:
    // MongoDB path
    return mongoInjectTemplatedCursor(paginatedQuery, cursorValue)
}
```

**Why:** All SQL-family databases share the same `sqlFormatCursorValue` helper which handles
quoting/casting. Only MongoDB needs a special injector because its cursor values are BSON
ObjectIDs, not SQL literals.

---

### Step 1.9 — Add schema checksum support

**File:** `backend/pkg/dbmanager/schema_manager.go` — function `getTableChecksums`

The schema manager hashes each table's structure to detect DDL changes and invalidate the LLM's
schema cache.

```go
func (sm *SchemaManager) getTableChecksums(ctx context.Context, db DBExecutor, dbType string) (map[string]string, error) {
    switch dbType {
    case constants.DatabaseTypePostgreSQL, constants.DatabaseTypeYugabyteDB,
        constants.DatabaseTypeTimescaleDB, constants.DatabaseTypeMyNewDB: // ← ADD
        // (identical block — get schema, hash each table definition)
        ...
    case constants.DatabaseTypeMySQL, constants.DatabaseTypeStarRocks:
        ...
    }
}
```

**Why:** Schema change detection is what triggers automatic LLM prompt and vector store updates
after a DDL operation. Without this, schema changes go undetected and the LLM gives stale answers
about table structure.

---

### Step 1.10 — Add schema-change trigger (DDL detection)

**File:** `backend/pkg/dbmanager/manager_executor.go` — post-commit goroutine in `ExecuteQuery`

After every committed query, a goroutine fires `OnSchemaChange` when a DDL was detected:

```go
go func() {
    time.Sleep(2 * time.Second) // wait for DDL to be visible
    switch conn.Config.Type {
    case constants.DatabaseTypePostgreSQL, constants.DatabaseTypeYugabyteDB,
        constants.DatabaseTypeTimescaleDB, constants.DatabaseTypeMyNewDB: // ← ADD
        if queryType == "DDL" || queryType == "ALTER" || queryType == "DROP" {
            if conn.OnSchemaChange != nil {
                conn.OnSchemaChange(conn.ChatID)
            }
        }
    case constants.DatabaseTypeMySQL, constants.DatabaseTypeStarRocks:
        // ...
    // For MongoDB, the trigger is CREATE_COLLECTION / DROP_COLLECTION
    }
}()
```

**Why:** The 2-second sleep gives the DB time to commit the DDL before the schema fetcher reads
it. `OnSchemaChange` then re-fetches and re-vectorises the schema. Without this hook the LLM
cache becomes stale after CREATE TABLE or ALTER TABLE.

---

### Step 1.11 — Add TestConnection support

**File:** `backend/pkg/dbmanager/manager_executor.go` — `TestConnection` function

`TestConnection` validates credentials *before* persisting a chat. It needs its own `case` so it
knows which default port and which driver to use.

```go
func (m *Manager) TestConnection(config *ConnectionConfig) error {
    switch config.Type {
    // ... existing cases ...
    case constants.DatabaseTypeMyNewDB:
        // For a Postgres-wire DB: fall through to the postgresql case
        // OR copy the postgresql case and change the default port
        port := "XXXX" // your DB's default port
        if config.Port != nil && *config.Port != "" {
            port = *config.Port
        }
        // Build DSN, call sql.Open("your-driver", dsn), db.Ping(), db.Close()
        // Handle SSL the same way the postgresql case does
    }
}
```

For wrapper DBs (e.g. TimescaleDB), the cleanest approach is to add the type to the parent's
existing case:

```go
case constants.DatabaseTypePostgreSQL, constants.DatabaseTypeYugabyteDB,
    constants.DatabaseTypeTimescaleDB, constants.DatabaseTypeMyNewDB: // ← ADD
    port := "5432" // or DB-specific default
    if config.Type == constants.DatabaseTypeMyNewDB {
        port = "XXXX"
    }
```

**Why:** `TestConnection` is the very first thing called when a user clicks "Test Connection" in
the UI. An unhandled type falls to the `default` case which returns "unsupported database type"
error. The user sees this immediately during setup.

---

### Step 1.12 — Update PostgresWrapper driver fallback (Postgres-wire DBs only)

**File:** `backend/pkg/dbmanager/db_wrapper.go` — `PostgresWrapper.GetSchema` and
`PostgresWrapper.GetTableChecksum`

`PostgresWrapper` is used for all PostgreSQL-wire databases. Its `GetSchema` and
`GetTableChecksum` methods look up the registered driver by a hardcoded key string. Extend the
fallback chain:

```go
// In PostgresWrapper.GetSchema and PostgresWrapper.GetTableChecksum
driver, exists := w.manager.drivers["postgresql"]
if !exists {
    driver, exists = w.manager.drivers["yugabytedb"]
    if !exists {
        driver, exists = w.manager.drivers["timescaledb"]
        if !exists {
            driver, exists = w.manager.drivers["mynewdb"] // ← ADD for Postgres-wire DBs
            if !exists {
                return nil, fmt.Errorf("driver not found")
            }
        }
    }
}
```

**Why:** A single `PostgresWrapper` instance handles any Postgres-wire DB. The fallback chain
lets the wrapper find the right registered driver regardless of which DB type opened the
connection. Without this, a TimescaleDB or YugabyteDB connection will fail to fetch its schema
if the `"postgresql"` driver key was not the one registered.

---

### Step 1.13 — Add default port to the chat execution service

**File:** `backend/internal/services/chat_execution_service.go`

When a saved chat is reconnected and its `port` field is empty (e.g. imported from an older
backup), this switch fills in a sensible default:

```go
switch chat.Connection.Type {
case constants.DatabaseTypePostgreSQL:
    defaultPort = "5432"
case constants.DatabaseTypeTimescaleDB:
    defaultPort = "5432" // TimescaleDB runs on standard PostgreSQL port
case constants.DatabaseTypeYugabyteDB:
    defaultPort = "5433"
case constants.DatabaseTypeMySQL:
    defaultPort = "3306"
case constants.DatabaseTypeStarRocks:
    defaultPort = "9030" // StarRocks FE MySQL-protocol query port
case constants.DatabaseTypeMyNewDB:  // ← ADD
    defaultPort = "XXXX"
// ...
}
```

**Why:** Without a default here the `Port` field stays empty, the DSN string becomes malformed,
and the DB driver returns a connection error. This is a subtle bug that only surfaces during
chat reconnect, not during the initial "Test Connection" flow.

---

### Step 1.14 — Add to the valid connection types list

**File:** `backend/internal/services/chat_crud_service.go`

```go
validTypes := []string{
    constants.DatabaseTypePostgreSQL,
    constants.DatabaseTypeYugabyteDB,
    constants.DatabaseTypeTimescaleDB,
    constants.DatabaseTypeMySQL,
    constants.DatabaseTypeStarRocks,
    constants.DatabaseTypeClickhouse,
    constants.DatabaseTypeMongoDB,
    constants.DatabaseTypeSpreadsheet,
    constants.DatabaseTypeGoogleSheets,
    constants.DatabaseTypeMyNewDB,  // ← ADD
}
```

**Why:** This slice is validated *before* the connection is persisted to MongoDB. Any type not
listed here is rejected with a `400 Bad Request`. It is a second-layer defence after DTO
binding validation (Step 1.15).

---

### Step 1.15 — Add to the DTO validation

**File:** `backend/internal/apis/dtos/chat.go`

```go
type CreateConnectionRequest struct {
    Type string `json:"type" binding:"required,oneof=postgresql yugabytedb timescaledb mysql starrocks clickhouse mongodb redis neo4j cassandra spreadsheet google_sheets mynewdb"`
    // ...
}
```

**Why:** `binding:"required,oneof=..."` is validated by the Gin framework at the HTTP boundary
before the request reaches any service logic. It is the *first* line of defence — invalid DB
types are rejected before any database connection is attempted.

> **Important:** This is a space-separated list of string literals, not Go constants. Use the
> actual lowercase string value you defined in `databases.go`.

---

### Step 1.16 — Add to visualization query wrapping

**File:** `backend/internal/services/visualization_service.go` — `wrapQueryWithLimit`

When visualizing data, result sets are automatically capped so the frontend doesn't receive
hundreds of thousands of rows.

```go
func wrapQueryWithLimit(query string, dbType string, limit int) string {
    switch strings.ToUpper(dbType) {
    case "MYSQL", "POSTGRESQL", "YUGABYTEDB", "CLICKHOUSE",
        "TIMESCALEDB", "STARROCKS", "MYNEWDB": // ← ADD
        // standard SQL: wrap in SELECT * FROM (...) AS t LIMIT N
        return fmt.Sprintf("SELECT * FROM (%s) AS _neobase_limit LIMIT %d", query, limit)
    case "MONGODB":
        // MongoDB: inject .limit(N)
    }
}
```

**Why:** Without this, a poorly written LLM-generated visualization query (e.g.
`SELECT * FROM events`) could return millions of rows, overwhelming the browser and causing
memory pressure in the backend. The correct SQL LIMIT dialect must be used per DB type.

---

### Step 1.17 — Add schema discovery query for embeddings

**File:** `backend/internal/constants/embedding.go` — `GetRagNoMatchingTablesFound`

When the vector search finds no matching schema, the LLM is instructed to discover the schema
manually using tool calls. The discovery query differs by DB:

```go
func GetRagNoMatchingTablesFound(dbType string) string {
    var discoveryStep string
    switch dbType {
    case DatabaseTypeMongoDB:
        discoveryStep = "1. Run `db.getCollectionNames()` ..."
    case DatabaseTypeClickhouse:
        discoveryStep = "1. Run `SHOW TABLES` ..."
    case DatabaseTypeMySQL, DatabaseTypeStarRocks:
        discoveryStep = "1. Run `SHOW TABLES` ..."
    case DatabaseTypeMyNewDB:  // ← ADD
        discoveryStep = "1. Run `<discovery query for MyNewDB>` ..."
    default:
        // PostgreSQL-family: use information_schema
        discoveryStep = "1. Run `SELECT table_name FROM information_schema.tables WHERE table_schema = 'public'` ..."
    }
    // ...
}
```

**Why:** If the new DB's schema discovery query is not handled, the LLM receives the PostgreSQL
`information_schema` query — which may not exist in all DB engines — and will fail to discover
tables, producing empty or confusing answers.

---

### Step 1.18 — Add embedding schema vectorization metadata

**File:** `backend/internal/services/vectorization_service.go` — `getDBTerminology`

The vectorization pipeline embeds schema information into Qdrant for semantic search. The
terminology struct enriches embedding context with DB-specific language:

```go
func getDBTerminology(dbType string) DBTerminology {
    switch dbType {
    case constants.DatabaseTypeTimescaleDB:
        return DBTerminology{
            EngineNote: "PostgreSQL extension optimised for time-series data; use time_bucket()",
        }
    case constants.DatabaseTypeMyNewDB:  // ← ADD
        return DBTerminology{
            EngineNote: "Brief one-liner about this DB for embedding context",
        }
    }
}
```

Also check the `GetSchemaDiscoveryQuery` function in `embedding.go`:

```go
func GetSchemaDiscoveryQuery(dbType string) string {
    switch dbType {
    case DatabaseTypeMySQL, DatabaseTypeStarRocks:
        return "SHOW TABLES"
    case DatabaseTypeMyNewDB:  // ← ADD if different from parent
        return "<table listing query>"
    default:
        return "SELECT table_name FROM information_schema.tables WHERE table_schema = 'public'"
    }
}
```

**Why:** The vectorization service runs in a background goroutine when a chat connects. It uses
these values to produce richer embedding strings (e.g. `"users table [PostgreSQL extension
optimised for time-series]"`). Better embedding strings → better semantic table search →
fewer "no matching tables found" fallbacks.

---

### Step 1.19 — Add dashboard instructions

**File:** `backend/internal/constants/dashboard.go` — `GetDashboardDBInstructions`

Dashboard widget generation uses a dedicated instruction set to guide the LLM on query patterns,
aggregations, and result limits:

```go
func GetDashboardDBInstructions(dbType string) string {
    switch dbType {
    case DatabaseTypePostgreSQL, DatabaseTypeYugabyteDB:
        return `...use standard SQL aggregations...`
    case DatabaseTypeTimescaleDB:
        return `...prefer time_bucket() for time-series rollups...`
    case DatabaseTypeMyNewDB:  // ← ADD
        return `Instructions specific to MyNewDB:
- Use <aggregation_function>() for cardinality.
- Filter on partition key columns first for performance.
- Default LIMIT 1000 for analytical queries.`
    }
}
```

**Why:** Dashboard queries are auto-generated without user input. If the LLM doesn't know the
DB's idiomatic aggregation functions and LIMIT syntax, generated dashboard widgets will either
time out or return partial data.

---

## Part 2 — Frontend Client (`client/src/`)

---

### Step 2.1 — Extend the TypeScript Connection type union

**File:** `client/src/types/chat.ts`

```ts
export interface Connection {
  // ...
  type: 'postgresql' | 'yugabytedb' | 'timescaledb' | 'mysql' | 'starrocks'
       | 'clickhouse' | 'mongodb' | 'redis' | 'neo4j' | 'spreadsheet'
       | 'google_sheets' | 'mynewdb';  // ← ADD
}
```

**Why:** This union is the TypeScript source of truth for the `type` field. TypeScript's
exhaustiveness checks propagate this to every component that switches on `connection.type`.
Without this addition, TypeScript silently accepts typos when you add the new type elsewhere.

---

### Step 2.2 — Add the database logo

1. Obtain a clear, square PNG or SVG for the database (official logos are usually on the project's
   GitHub org page, press kit, or official website at ~512×512px).

2. Place the file in **both** public directories:
   - `client/public/mynewdb-logo.png`
   - `neobase-landing/public/mynewdb-logo.png` (used in Step 4.1)

3. **File:** `client/src/components/icons/DatabaseLogos.tsx`

```ts
interface DatabaseLogoProps {
  type: '...' | 'mynewdb';  // ← ADD to union
}

const databaseLogos: Record<...> = {
  // ...
  mynewdb: `${import.meta.env.VITE_FRONTEND_BASE_URL}mynewdb-logo.png`,  // ← ADD
};
```

**Why:** `DatabaseLogo` is the single component rendering a DB icon. It is imported by the
Sidebar, ChatHeader, and this addition covers both automatically. `VITE_FRONTEND_BASE_URL` ensures
the path is correct in both local development and deployed environments.

---

### Step 2.3 — Add to the connection modal dropdown

**File:** `client/src/components/modals/ConnectionModal.tsx`

```tsx
{[
  { value: 'postgresql',  label: 'PostgreSQL' },
  { value: 'timescaledb', label: 'TimescaleDB' },
  { value: 'yugabytedb',  label: 'YugabyteDB' },
  { value: 'mysql',       label: 'MySQL' },
  { value: 'starrocks',   label: 'StarRocks' },
  { value: 'mynewdb',     label: 'MyNewDB' },  // ← ADD — use human-readable label
  { value: 'clickhouse',  label: 'ClickHouse' },
  { value: 'mongodb',     label: 'MongoDB' },
  // ...
].map(option => (
  <option key={option.value} value={option.value}>{option.label}</option>
))}
```

**Why:** This dropdown is how users select the database type when creating a new connection.
Without an entry here the new DB is completely inaccessible to end users regardless of all other
changes.

---

### Step 2.4 — Update BasicConnectionTab

**File:** `client/src/components/modals/components/BasicConnectionTab.tsx`

Four changes in this file:

**a) Default port** — `getDefaultPort(dbType)`:
```ts
case 'mynewdb':
    return 'XXXX';
```

**b) URI config** — `getUriConfig(dbType)` — controls the URI paste-to-autofill feature:
```ts
case 'mynewdb':
    return {
        label: 'MyNewDB Connection URI',
        placeholder: 'protocol://username:password@host:port/database',
        description: 'Paste your MyNewDB connection string to auto-fill fields'
    };
```

**c) URI builder** — `buildConnectionUri(data)` — generates the URI from form fields:
```ts
case 'mynewdb':
    uri = `protocol://${username}:${password}@${host}:${port}/${database}`;
    break;
```

**d) URI parser** — `parseConnectionUri(uri, dbType)` — reverse of the builder, used when
pasting a URI:
```ts
case 'mynewdb':
    parsedData = parseMyNewDBUri(uri); // or reuse parsePostgreSQLUri / parseMySQLUri
    break;
```

**e) SSL hint** — inline JSX in the Connection URI help text:
```tsx
{formData.type === 'mynewdb' && ' Supports <ssl-param> parameter (e.g., ?<ssl-param>=value).'}
```

**Why:** The basic tab is the primary connection form. The default port prefills the port field so
users don't need to look it up. The URI autofill is the most common "power user" path — pasting
a connection string from an existing tool or cloud dashboard. Both flows must know the DB's URI
format.

---

### Step 2.5 — Update SSHConnectionTab

**File:** `client/src/components/modals/components/SSHConnectionTab.tsx`

Apply the **same five changes** as Step 2.4: `getDefaultPort`, `getUriConfig`,
`buildConnectionUri`, `parseConnectionUri`, and the SSL hint.

The SSH tab code duplicates the URI logic because it runs a completely separate connection flow
(all traffic is tunnelled through SSH before reaching the DB). Both tabs must be consistent.

**Why:** If the SSH tab doesn't know the default port, a user connecting via SSH tunnel must
manually look up and type the port — a source of frustrating connection failures.

---

### Step 2.6 — Update Sidebar

**File:** `client/src/components/chat/Sidebar.tsx`

Three changes:

**a) Local `Connection` interface** — extend the `type` union:
```ts
interface Connection {
  type: 'postgresql' | 'yugabytedb' | 'timescaledb' | 'mysql' | 'starrocks'
       | 'clickhouse' | 'mongodb' | 'redis' | 'neo4j' | 'spreadsheet'
       | 'google_sheets' | 'mynewdb';  // ← ADD
}
```

**b) `DatabaseLogo` type cast** — used when rendering the connection icon:
```tsx
<DatabaseLogo
  type={connection.connection.type as
    'postgresql' | 'yugabytedb' | 'timescaledb' | 'mysql' | 'starrocks'
    | 'clickhouse' | 'mongodb' | 'redis' | 'neo4j' | 'spreadsheet'
    | 'google_sheets' | 'mynewdb'}  // ← ADD
  ...
/>
```

**c) Display name ternary chain** — the human-readable label shown under the icon:
```tsx
: connection.connection.type === 'timescaledb'
    ? 'TimescaleDB'
    : connection.connection.type === 'mynewdb'  // ← ADD
        ? 'MyNewDB'
        : connection.connection.type === 'mysql'
            ? 'MySQL'
```

**Why:** The sidebar is the main navigation element users interact with. Without these changes,
the new DB's connection shows a missing icon (runtime logo load error), no human-readable name
(falls through to 'Unknown'), and TypeScript compile errors on the type cast.

---

### Step 2.7 — Update ChatHeader logo cast

**File:** `client/src/components/chat/ChatHeader.tsx`

```tsx
<DatabaseLogo
  type={chat.connection.type as
    "postgresql" | "yugabytedb" | "timescaledb" | "mysql" | "starrocks"
    | "mongodb" | "redis" | "clickhouse" | "neo4j" | "mynewdb"}  // ← ADD
  size={32}
  className="transition-transform hover:scale-110"
/>
```

**Why:** The type cast narrows `string` to the `DatabaseLogoProps['type']` union. Without adding
the new type, TypeScript raises a compile error because `"mynewdb"` is not in the cast union.

---

## Part 3 — Docker Compose (`docker-compose/`)

---

### Step 3.1 — Add an example service

**File:** `docker-compose/docker-compose-exampledbs.yml`

```yaml
  neobase-example-mynewdb:
    image: vendor/mynewdb:latest
    container_name: neobase-example-mynewdb
    restart: always
    ports:
      - XXXX:XXXX        # host port : container port
    environment:
      # Minimal credentials — only what the DB requires to start
      DB_USER: admin
      DB_PASSWORD: admin
      DB_NAME: testdb
    volumes:
      - mynewdb-data:/var/lib/mynewdb
    networks:
      - neobase-network
```

Also add the named volume at the bottom of the file:
```yaml
volumes:
  # ...
  mynewdb-data:
```

**Why:** The example-db compose file is used by contributors and users who want a zero-config
local instance for testing. It avoids the need to install and configure a native DB just to test
the NeoBase integration. It is also used in CI if integration tests are added later.

> **Port conflicts:** Check the existing services. Use a unique host-side port. For example,
> TimescaleDB uses `5433:5432` to avoid conflicting with the main postgres at `5432:5432`.

---

## Part 4 — Landing Page (`neobase-landing/src/`)

---

### Step 4.1 — Add logo to the supported-technologies marquee

**File:** `neobase-landing/src/components/SupportedTechnologiesSection.tsx`

```ts
const allLogos: Logo[] = [
  { name: 'PostgreSQL',   src: '/postgresql-logo.png',  alt: 'PostgreSQL Logo' },
  { name: 'MySQL',        src: '/mysql-logo.png',         alt: 'MySQL Logo' },
  // ...
  { name: 'MyNewDB',      src: '/mynewdb-logo.png',       alt: 'MyNewDB Logo' },  // ← ADD
];
```

The logo file (`mynewdb-logo.png`) must already be in `neobase-landing/public/` — placed there in
Step 2.2.

**Why:** The landing page shows prospective users which databases are supported. Failing to update
it means the feature exists but is invisible to anyone discovering NeoBase for the first time.

---

## Part 5 — Testing

---

### 5.1 — Start the example service

```bash
# From the repo root
docker-compose -f docker-compose/docker-compose-exampledbs.yml up -d neobase-example-mynewdb

# Confirm it started
docker logs neobase-example-mynewdb
```

---

### 5.2 — Build the backend

```bash
cd backend
go build ./...
```

This must produce **zero errors**. If there are type errors or missing constants, fix them before
continuing.

---

### 5.3 — TypeScript check

```bash
cd client
npx tsc --noEmit
```

This must produce **zero errors**. Common failures:
- Missing DB type in a union → add to the relevant `type` field / cast
- `DatabaseLogos` key type error → ensure the type is in `DatabaseLogoProps['type']`

---

### 5.4 — Manual connection test

1. Start the full NeoBase stack: `docker-compose -f docker-compose/docker-compose-local.yml up`
2. Open the app in the browser.
3. Click **New Connection** → select **MyNewDB** from the dropdown.
4. Fill in:
   - Host: `localhost`
   - Port: `XXXX` (or leave empty and verify it pre-fills the correct default)
   - Database: `testdb`
   - Username / Password as defined in the compose service
5. Click **Test Connection** → should show a green success message.
6. Click **Connect** → should open the chat.

---

### 5.5 — Schema test

In the chat, ask:

```
Show me all tables in this database
```

Expected: The LLM issues the correct schema-discovery query for MyNewDB and lists the tables.

If it fails or shows a wrong query, recheck:
- `GetRagNoMatchingTablesFound` in `embedding.go` (Step 1.17)
- `getDBTerminology` / `GetSchemaDiscoveryQuery` in `vectorization_service.go` (Step 1.18)

---

### 5.6 — Query generation test

Ask a simple question that exercises the LLM prompt:

```
How many rows are in each table?
```

Expected: The LLM generates syntactically correct MyNewDB SQL (not PostgreSQL or MySQL syntax).

If the syntax is wrong, recheck the prompt in Step 1.2 and the `getDatabasePrompt` dispatch in
Step 1.3.

---

### 5.7 — Non-tech mode test

In the chat settings, enable **Non-Technical Mode**. Then ask:

```
What's in this database?
```

Expected: The assistant response contains no SQL jargon. If it does, recheck
`getNonTechModeInstructions` (Step 1.3b).

---

### 5.8 — Dashboard / visualization test

Open **Dashboards** and click **Add Widget**.

Expected: The LLM generates valid MyNewDB queries for the selected metric. If it generates
PostgreSQL-specific syntax, recheck `GetVisualizationPrompt` (Step 1.3c) and
`GetDashboardDBInstructions` (Step 1.19).

---

### 5.9 — Schema change detection test

In the chat, run:

```sql
CREATE TABLE mynewdb_test_schema_detection (id INT, name TEXT);
```

Then immediately ask:

```
Describe the mynewdb_test_schema_detection table
```

Expected: The LLM describes the new table correctly. If it says the table doesn't exist, recheck
the `OnSchemaChange` trigger in `manager_executor.go` (Step 1.10) and the query classification
(Step 1.7).

---

### 5.10 — SSL connection test (optional)

If the DB supports SSL:

1. In the connection form, enable the SSL toggle.
2. Verify the SSL hint text appears (Step 2.4e).
3. Set up a test certificate and confirm the connection succeeds with SSL.

---

## Full Checklist

Paste this into your PR description and tick each box before requesting review.

### Backend
- [ ] `constants/databases.go` — new `DatabaseType*` constant defined
- [ ] `constants/mynewdb.go` — full prompt or extension constants created
- [ ] `constants/llms.go` — `getDatabasePrompt`, `getNonTechModeInstructions`, `GetVisualizationPrompt` updated
- [ ] `di/modules.go` — driver + fetcher registered; `LLMDBConfig` added to all 4 LLM provider blocks (OpenAI, Gemini, Claude, Ollama)
- [ ] `pkg/dbmanager/manager_crud.go` — `registerDefaultDrivers` + `RegisterFetcher` + `GetConnection` updated
- [ ] `pkg/dbmanager/mynewdb_driver.go` — driver created (native DB only)
- [ ] `pkg/dbmanager/mynewdb_wrapper.go` — wrapper created (native DB with custom wire protocol only)
- [ ] `pkg/dbmanager/query_validator.go` — validator mapping added
- [ ] `constants/query_classification.go` — classification map entry added
- [ ] `pkg/dbmanager/cursor_utils.go` — SQL cursor case updated
- [ ] `pkg/dbmanager/schema_manager.go` — checksum case added
- [ ] `pkg/dbmanager/manager_executor.go` — schema trigger case added; `TestConnection` case added
- [ ] `pkg/dbmanager/db_wrapper.go` — fallback chain updated (Postgres-wire wrapper DBs only)
- [ ] `services/chat_execution_service.go` — default port added
- [ ] `services/chat_crud_service.go` — added to `validTypes` slice
- [ ] `apis/dtos/chat.go` — added to `oneof` binding tag
- [ ] `services/visualization_service.go` — `wrapQueryWithLimit` case added
- [ ] `constants/embedding.go` — `GetRagNoMatchingTablesFound` case added
- [ ] `services/vectorization_service.go` — DB terminology added
- [ ] `constants/dashboard.go` — dashboard instructions added
- [ ] `go build ./...` passes with zero errors

### Frontend Client
- [ ] `types/chat.ts` — `Connection.type` union extended
- [ ] Logo PNG/SVG placed in `client/public/`
- [ ] `components/icons/DatabaseLogos.tsx` — type union + logo map updated
- [ ] `components/modals/ConnectionModal.tsx` — dropdown option added
- [ ] `components/modals/components/BasicConnectionTab.tsx` — default port, URI config, URI builder, URI parser, SSL hint
- [ ] `components/modals/components/SSHConnectionTab.tsx` — same five changes as BasicConnectionTab
- [ ] `components/chat/Sidebar.tsx` — local interface, logo cast, display name chain
- [ ] `components/chat/ChatHeader.tsx` — logo cast updated
- [ ] `npx tsc --noEmit` passes with zero errors

### Docker Compose
- [ ] `docker-compose/docker-compose-exampledbs.yml` — service definition added; named volume added

### Landing Page
- [ ] Logo PNG/SVG placed in `neobase-landing/public/`
- [ ] `neobase-landing/src/components/SupportedTechnologiesSection.tsx` — logo entry added

### Testing
- [ ] `docker-compose up neobase-example-mynewdb` starts without errors
- [ ] `go build ./...` clean
- [ ] `npx tsc --noEmit` clean
- [ ] Test Connection flow succeeds in the UI
- [ ] Schema is fetched and displayed correctly
- [ ] LLM generates syntactically valid MyNewDB queries
- [ ] Non-tech mode produces jargon-free responses
- [ ] Dashboard widget query generation works
- [ ] Schema change detection fires after a DDL operation


---

## Part 1 — Backend (`backend/`)

### 1.1 Register the DB type constant

**File:** `backend/internal/constants/databases.go`

Add a `DatabaseType*` constant. This string becomes the canonical identifier used everywhere in the system.

```go
const (
    DatabaseTypePostgreSQL  = "postgresql"
    DatabaseTypeMySQL       = "mysql"
    // ...
    DatabaseTypeMyNewDB     = "mynewdb"   // ← add here
)
```

**Why:** A single constant avoids typos across ~20 files. Every switch statement, map key, and DTO validation references this value.

---

### 1.2 Write the LLM prompt

#### 1.2a Native DB — new prompt file

**File:** `backend/internal/constants/mynewdb.go` *(create)*

```go
package constants

const MyNewDBPrompt = `You are NeoBase AI, an expert <MyNewDB> database assistant.
...
`

const MyNewDBVisualizationPrompt = `...`
```

Model the file on `postgresql.go` or `mysql.go`. Include:
- Query syntax rules and dialect quirks
- Schema introspection conventions
- Things the LLM must **never** do (e.g. unsafe mutations)
- Performance hints

#### 1.2b Wrapper DB — extension constants

If the new DB re-uses a parent driver, create a small extension file instead:

```go
package constants

// MyNewDBExtensions is appended to <ParentDB>Prompt.
const MyNewDBExtensions = `
---
### MyNewDB-Specific Rules (append to <Parent> rules above)
...
`

const MyNewDBVisualizationExtensions = `...`
```

**Why:** Keeping the parent prompt intact means bug-fixes to the parent automatically benefit wrapper DBs. The extension only captures the *delta*.

---

### 1.3 Wire the prompt into the LLM dispatcher

**File:** `backend/internal/constants/llms.go`

**1.3a** `getDatabasePrompt`:
```go
case DatabaseTypeMyNewDB:
    return MyNewDBPrompt                   // native
    // OR for wrapper:
    return ParentPrompt + MyNewDBExtensions
```

**1.3b** `getNonTechModeInstructions`:
```go
// Group with the parent if wire-compatible, or add its own case.
case DatabaseTypeMyNewDB:
    return baseInstructions + getMyNewDBNonTechInstructions()
```

**1.3c** `GetVisualizationPrompt`:
```go
case DatabaseTypeMyNewDB:
    return MyNewDBVisualizationPrompt          // native
    // OR:
    return ParentVisualizationPrompt + MyNewDBVisualizationExtensions
```

**Why:** These three functions are the single dispatch point for system prompts. Missing a case means the LLM falls back to PostgreSQL, which produces wrong syntax for the user's database.

---

### 1.4 Register the driver and schema fetcher

#### 1.4a DI container (production path)

**File:** `backend/internal/di/modules.go`

```go
// Driver registration — tells the manager how to open a connection
manager.RegisterDriver(constants.DatabaseTypeMyNewDB, dbmanager.NewMyNewDBDriver())
// For wrapper DBs: reuse the parent driver
manager.RegisterDriver(constants.DatabaseTypeMyNewDB, dbmanager.NewPostgresDriver())

// Schema fetcher — tells the manager how to introspect the schema
manager.RegisterFetcher(constants.DatabaseTypeMyNewDB, func(db dbmanager.DBExecutor) dbmanager.SchemaFetcher {
    return &dbmanager.MyNewDBDriver{}
    // For wrapper: return &dbmanager.PostgresDriver{}
    // For MySQL wrapper: return dbmanager.NewMySQLSchemaFetcher(db)
})
```

Also add a `DBConfig` entry for every LLM provider block (OpenAI, Gemini, Claude, Ollama):
```go
{
    DBType:       constants.DatabaseTypeMyNewDB,
    Schema:       constants.GetLLMResponseSchema(constants.OpenAI, constants.DatabaseTypeMyNewDB),
    SystemPrompt: constants.GetSystemPrompt(constants.OpenAI, constants.DatabaseTypeMyNewDB, false),
},
```
Repeat for `Gemini`, `Claude`, and `Ollama` blocks.

**Why:** `LLMDBConfig` pre-loads the system prompt and response schema per DB type so the LLM manager can quickly look them up at request time without re-computing them.

#### 1.4b Lower-level manager (fallback / lazy-load path)

**File:** `backend/pkg/dbmanager/manager_crud.go`

The `registerDefaultDrivers` function mirrors the DI registration for contexts that bypass DI:
```go
m.RegisterDriver("mynewdb", NewMyNewDBDriver())

m.RegisterFetcher("mynewdb", func(db DBExecutor) SchemaFetcher {
    return &MyNewDBDriver{}
})
```

Also update the `GetConnection` switch so requests for the new DB type receive the correct wrapper:
```go
case constants.DatabaseTypeMyNewDB:
    // return NewMyNewDBWrapper(db, m, chatID)
    // For Postgres-wire: fall through to the PostgreSQL case
```

---

### 1.5 Write the driver (native DBs only)

Create `backend/pkg/dbmanager/mynewdb_driver.go`.

At minimum the driver must implement `SchemaFetcher`:
```go
func (d *MyNewDBDriver) GetSchema(ctx context.Context, db DBExecutor, selectedTables []string) (*SchemaInfo, error)
func (d *MyNewDBDriver) GetTableChecksum(ctx context.Context, db DBExecutor, table string) (string, error)
```

If the DB needs a connection wrapper (i.e. it is not wire-compatible with an existing one), also create `mynewdb_wrapper.go` implementing `DBExecutor`.

**Why:** The driver is the only place that understands the DB's metadata API (e.g. `information_schema`, `SHOW TABLES`, aggregated collection stats). Everything above it operates on the generic `SchemaInfo` struct.

---

### 1.6 Add query validation

**File:** `backend/pkg/dbmanager/query_validator.go`

```go
case "mynewdb":
    return NewSQLQueryValidator("postgresql") // or "mysql", "mongodb", etc.
```

**Why:** Query validation uses a dialect-specific lexer. Mapping to the closest parent dialect gives free protection against destructive queries without needing a bespoke parser.

---

### 1.7 Add query classification

**File:** `backend/internal/constants/query_classification.go`

```go
var queryClassificationMap = map[string]QueryClassification{
    // ...
    DatabaseTypeMyNewDB: PostgreSQLQueryClassification, // or MySQLQueryClassification
}
```

**Why:** `GetQueryClassification` is used to determine whether a query is a DDL, DML, or read — which controls schema-change triggers and audit logging.

---

### 1.8 Add cursor / pagination support

**File:** `backend/pkg/dbmanager/cursor_utils.go`

```go
case constants.DatabaseTypePostgreSQL, constants.DatabaseTypeMySQL,
    constants.DatabaseTypeYugabyteDB, constants.DatabaseTypeTimescaleDB,
    constants.DatabaseTypeStarRocks, constants.DatabaseTypeMyNewDB, // ← add
    constants.DatabaseTypeClickhouse:
```

**Why:** Cursor-based pagination injects a `{{cursor_value}}` placeholder into paginated queries. All SQL-like databases share the same `sqlFormatCursorValue` helper; only MongoDB needs special handling.

---

### 1.9 Add schema checksum support

**File:** `backend/pkg/dbmanager/schema_manager.go` — `getTableChecksums`

```go
case constants.DatabaseTypeMyNewDB:
    // identical block to the closest parent
```

**Why:** The schema manager hashes each table's structure to detect schema changes and trigger LLM cache invalidation. Every DB type needs this even if the implementation is identical to an existing one.

---

### 1.10 Add schema trigger (DDL detection)

**File:** `backend/pkg/dbmanager/manager_executor.go` — `ExecuteQuery` goroutine

```go
case constants.DatabaseTypeMyNewDB:
    if queryType == "DDL" || queryType == "ALTER" || queryType == "DROP" {
        if conn.OnSchemaChange != nil {
            conn.OnSchemaChange(conn.ChatID)
        }
    }
```

**Why:** After a DDL query executes, the schema cache must be invalidated. This goroutine fires `OnSchemaChange` after a 2-second delay so the schema fetcher has time to see the committed change.

---

### 1.11 Add default port to TestConnection

**File:** `backend/pkg/dbmanager/manager_executor.go` — `TestConnection`

```go
case constants.DatabaseTypeMyNewDB:
    port := "XXXX" // default port
    // ... copy pattern from mysql or postgresql case
```

**Why:** `TestConnection` is called before a chat is created to validate credentials. It needs to know the default port so it can connect even when the user didn't specify one.

---

### 1.12 Update PostgresWrapper driver fallback (Postgres-wire DBs only)

**File:** `backend/pkg/dbmanager/db_wrapper.go` — `PostgresWrapper.GetSchema` and `PostgresWrapper.GetTableChecksum`

The fallback chain tries `"postgresql"` → `"yugabytedb"` → `"timescaledb"`. Add any new Postgres-wire DB here:

```go
driver, exists = w.manager.drivers["mynewdb"]
if !exists {
    return nil, fmt.Errorf("driver not found")
}
```

**Why:** A single `PostgresWrapper` instance may be used for any Postgres-wire DB. The fallback chain lets it find the right registered driver regardless of which DB type was actually connected.

---

### 1.13 Add default port to the chat execution service

**File:** `backend/internal/services/chat_execution_service.go`

```go
case constants.DatabaseTypeMyNewDB:
    defaultPort = "XXXX"
```

**Why:** When a saved chat is reconnected and its port field is empty, this switch fills in the correct default so the driver can open the connection.

---

### 1.14 Add to the valid connection types list

**File:** `backend/internal/services/chat_crud_service.go`

```go
validTypes := []string{
    // ...
    constants.DatabaseTypeMyNewDB,
}
```

**Why:** This slice is the authoritative whitelist checked before creating or updating a chat. Any type not listed here is rejected with a validation error.

---

### 1.15 Add to the DTO validation

**File:** `backend/internal/apis/dtos/chat.go`

```go
Type string `json:"type" binding:"required,oneof=postgresql yugabytedb mysql ... mynewdb"`
```

**Why:** `binding:"required,oneof=..."` is validated by the Gin framework at the HTTP boundary before the request reaches any service logic. It is the first line of defence against invalid type values.

---

### 1.16 Add to visualization query wrapping

**File:** `backend/internal/services/visualization_service.go` — `wrapQueryWithLimit`

```go
case "MYNEWDB":
    // return standard SQL LIMIT wrapper, or a DB-specific one
```

**Why:** Visualization queries are automatically wrapped with a `LIMIT` to prevent huge result sets from being returned to the frontend. The DB type determines which LIMIT dialect to use.

---

### 1.17 Add to embedding / schema vectorization

**File:** `backend/internal/constants/embedding.go`

```go
case DatabaseTypeMyNewDB:
    return "SHOW TABLES"   // or equivalent discovery query
```

**File:** `backend/internal/services/vectorization_service.go` — `getDBTerminology`

```go
case constants.DatabaseTypeMyNewDB:
    return DBTerminology{
        EngineNote: "Brief description for the embedding context",
        // ...
    }
```

**Why:** The vectorization pipeline embeds schema information into the vector store for semantic search. It needs to know how to discover tables and what terminology to use in the embedding context string.

---

### 1.18 Add dashboard instructions

**File:** `backend/internal/constants/dashboard.go` — `GetDashboardDBInstructions`

```go
case DatabaseTypeMyNewDB:
    return `...DB-specific dashboard query guidance...`
```

**Why:** Dashboard widget generation uses a separate instruction set that gives the LLM hints about which aggregation patterns, date functions, and LIMIT strategies to use for this specific DB.

---

## Part 2 — Frontend Client (`client/src/`)

### 2.1 Extend the TypeScript type union

**File:** `client/src/types/chat.ts`

```ts
type: 'postgresql' | 'yugabytedb' | ... | 'mynewdb';
```

**Why:** The `Connection.type` field is used throughout the component tree. Adding it to the union gives TypeScript exhaustiveness checks that catch any case you forgot to handle.

---

### 2.2 Add the DB logo

1. Place `mynewdb-logo.png` (or `.svg`) in:
   - `client/public/`
   - `neobase-landing/public/`

2. **File:** `client/src/components/icons/DatabaseLogos.tsx`

```ts
interface DatabaseLogoProps {
  type: '...' | 'mynewdb';
}

const databaseLogos = {
  // ...
  mynewdb: `${import.meta.env.VITE_FRONTEND_BASE_URL}mynewdb-logo.png`,
};
```

**Why:** `DatabaseLogo` is the single component responsible for rendering a DB type icon. It is used in the Sidebar, ChatHeader, and ConnectionModal. Adding the type here gives it logo coverage everywhere at once.

---

### 2.3 Add to the connection modal dropdown

**File:** `client/src/components/modals/ConnectionModal.tsx`

```tsx
{ value: 'mynewdb', label: 'MyNewDB' },
```

**Why:** This is the dropdown the user sees when creating a new connection. Without this entry the user has no way to select the new DB type.

---

### 2.4 Update BasicConnectionTab

**File:** `client/src/components/modals/components/BasicConnectionTab.tsx`

**Default port** (`getDefaultPort`):
```ts
case 'mynewdb':
    return 'XXXX';
```

**URI config** (`getUriConfig`):
```ts
case 'mynewdb':
    return {
        label: 'MyNewDB Connection URI',
        placeholder: 'protocol://username:password@host:port/database',
        description: 'Paste your MyNewDB connection string to auto-fill fields'
    };
```

**SSL hint** (inline JSX below the URI field):
```tsx
{formData.type === 'mynewdb' && ' Supports <ssl-param> parameter (e.g., ?<ssl-param>=value).'}
```

**URI builder** (`buildConnectionUri`):
```ts
case 'mynewdb':
    uri = `protocol://${username}:${password}@${host}:${port}/${database}`;
    break;
```

**Why:** The connection form must prefill sensible defaults so users don't need to look up ports or URI formats. The URI auto-fill dramatically reduces connection setup friction.

---

### 2.5 Update SSHConnectionTab

**File:** `client/src/components/modals/components/SSHConnectionTab.tsx`

Apply the same four changes as section 2.4 — `getDefaultPort`, `getUriConfig`, `buildConnectionUri`, `parseConnectionUri`, and the SSL hint paragraph.

**Why:** The SSH tunnel tab is a parallel connection form that duplicates the URI logic. Both tabs must stay in sync.

---

### 2.6 Update Sidebar

**File:** `client/src/components/chat/Sidebar.tsx`

1. Extend the local `Connection` interface type union.
2. Extend the `DatabaseLogo` type cast.
3. Add a display name branch to the ternary chain:
```tsx
: connection.connection.type === 'mynewdb'
    ? 'MyNewDB'
```

**Why:** The sidebar renders the DB icon and human-readable name for every saved connection. Without these additions the new DB type falls through to "Unknown".

---

### 2.7 Update ChatHeader

**File:** `client/src/components/chat/ChatHeader.tsx`

```tsx
<DatabaseLogo
  type={chat.connection.type as "postgresql" | ... | "mynewdb"}
  ...
/>
```

**Why:** The chat header shows the DB logo next to the database name. The type cast must include the new type or TypeScript raises a type error.

---

## Part 3 — Docker Compose (`docker-compose/`)

### 3.1 Add an example service

**File:** `docker-compose/docker-compose-exampledbs.yml`

```yaml
neobase-example-mynewdb:
  image: vendor/mynewdb:latest
  container_name: neobase-example-mynewdb
  restart: always
  ports:
    - XXXX:XXXX
  environment:
    # Minimal credentials for local testing
    DB_USER: admin
    DB_PASSWORD: admin
    DB_NAME: testdb
  volumes:
    - mynewdb-data:/var/lib/mynewdb
  networks:
    - neobase-network
```

Also add the named volume:
```yaml
volumes:
  mynewdb-data:
```

**Why:** The example-db compose file gives contributors and users a zero-config local instance for testing connections. It avoids the need to install a native DB just to test the integration.

---

## Part 4 — Landing Page (`neobase-landing/src/`)

### 4.1 Add logo to the marquee

**File:** `neobase-landing/src/components/SupportedTechnologiesSection.tsx`

```ts
const allLogos: Logo[] = [
  // ...
  { name: 'MyNewDB', src: '/mynewdb-logo.png', alt: 'MyNewDB Logo' },
];
```

The logo file (`mynewdb-logo.png` / `.svg`) must already be in `neobase-landing/public/` (placed in Step 2.2 above).

**Why:** The landing page shows prospective users which databases are supported. Keeping it in sync with the actual implementation avoids misleading users.

---

## Checklist

Use this as a PR checklist. Check each item before opening for review.

### Backend
- [ ] `constants/databases.go` — new `DatabaseType*` constant
- [ ] `constants/mynewdb.go` — prompt constant(s) created
- [ ] `constants/llms.go` — `getDatabasePrompt`, `getNonTechModeInstructions`, `GetVisualizationPrompt` updated
- [ ] `di/modules.go` — driver + fetcher registered; DBConfig added to all 4 LLM provider blocks
- [ ] `pkg/dbmanager/manager_crud.go` — `registerDefaultDrivers` + `RegisterFetcher` + `GetConnection`
- [ ] `pkg/dbmanager/mynewdb_driver.go` — driver created (native DB only)
- [ ] `pkg/dbmanager/query_validator.go` — validator mapping added
- [ ] `constants/query_classification.go` — classification map entry added
- [ ] `pkg/dbmanager/cursor_utils.go` — SQL cursor case updated
- [ ] `pkg/dbmanager/schema_manager.go` — checksum case added
- [ ] `pkg/dbmanager/manager_executor.go` — schema trigger case + TestConnection case
- [ ] `pkg/dbmanager/db_wrapper.go` — fallback chain updated (wrapper DBs only)
- [ ] `services/chat_execution_service.go` — default port added
- [ ] `services/chat_crud_service.go` — added to `validTypes`
- [ ] `apis/dtos/chat.go` — added to `oneof` binding
- [ ] `services/visualization_service.go` — `wrapQueryWithLimit` case added
- [ ] `constants/embedding.go` — schema discovery query added
- [ ] `services/vectorization_service.go` — DB terminology added
- [ ] `constants/dashboard.go` — dashboard instructions added
- [ ] `go build ./...` passes with zero errors

### Frontend Client
- [ ] `types/chat.ts` — type union extended
- [ ] Logo file in `client/public/`
- [ ] `components/icons/DatabaseLogos.tsx` — type + logo map updated
- [ ] `components/modals/ConnectionModal.tsx` — dropdown option added
- [ ] `components/modals/components/BasicConnectionTab.tsx` — port, URI config, SSL hint, URI builder
- [ ] `components/modals/components/SSHConnectionTab.tsx` — same as above + `parseConnectionUri`
- [ ] `components/chat/Sidebar.tsx` — interface, logo cast, display name
- [ ] `components/chat/ChatHeader.tsx` — logo cast updated
- [ ] `npx tsc --noEmit` passes with zero errors

### Docker Compose
- [ ] `docker-compose/docker-compose-exampledbs.yml` — service + named volume added

### Landing Page
- [ ] Logo file in `neobase-landing/public/`
- [ ] `neobase-landing/src/components/SupportedTechnologiesSection.tsx` — logo added to `allLogos`
