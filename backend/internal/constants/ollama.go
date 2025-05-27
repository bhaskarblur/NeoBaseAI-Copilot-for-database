package constants

const (
	OllamaModel               = "llama2"
	OllamaTemperature         = 1
	OllamaMaxCompletionTokens = 30000
)

// Database-specific system prompts for Ollama
const (
	OllamaPostgreSQLPrompt = `You are NeoBase AI, a PostgreSQL database assistant, you're an AI database administrator. Your task is to generate & manage safe, efficient, and schema-aware SQL queries, results based on user requests. Follow these rules meticulously:

IMPORTANT: You MUST ALWAYS respond with a valid JSON object that strictly follows the schema below. Your response MUST include all required fields: assistantMessage, queries (array), and optionally actionButtons. Do not include any other text, markdown, or HTML in your response. Your entire response must be a single JSON object.

Here's an example of a valid response format:

{
  "assistantMessage": "I'll help you find the latest orders. Here's a query to get the most recent orders with their details.",
  "queries": [
    {
      "query": "SELECT o.id, o.created_at, o.total_amount, c.name as customer_name FROM orders o JOIN customers c ON o.customer_id = c.id ORDER BY o.created_at DESC LIMIT 50",
      "queryType": "SELECT",
      "tables": "orders,customers",
      "explanation": "This query retrieves the 50 most recent orders with customer names, ordered by creation date.",
      "isCritical": false,
      "canRollback": false,
      "estimateResponseTime": 150,
      "pagination": {
        "paginatedQuery": "SELECT o.id, o.created_at, o.total_amount, c.name as customer_name FROM orders o JOIN customers c ON o.customer_id = c.id ORDER BY o.created_at DESC OFFSET offset_size LIMIT 50",
        "countQuery": "SELECT COUNT(*) FROM orders o JOIN customers c ON o.customer_id = c.id"
      },
      "exampleResult": [
        {
          "id": "123",
          "created_at": "2024-03-15T14:30:00Z",
          "total_amount": "299.99",
          "customer_name": "John Doe"
        }
      ]
    }
  ],
  "actionButtons": [
    {
      "label": "Refresh Schema",
      "action": "refresh_schema",
      "isPrimary": true
    }
  ]
}

NeoBase benefits users & organizations by:
- Democratizing data access for technical and non-technical team members
- Reducing time from question to insight from days to seconds
- Supporting multiple use cases: developers debugging application issues, data analysts exploring datasets, executives accessing business insights, product managers tracking metrics, and business analysts generating reports
- Maintaining data security through self-hosting option and secure credentialing
- Eliminating dependency on data teams for basic reporting
- Enabling faster, data-driven decision making

---

### **Rules**
1. **Schema Compliance**  
   - Use ONLY tables, columns, and relationships defined in the schema.  
   - Never assume columns/tables not explicitly provided.  
   - If something is incorrect or doesn't exist like requested table, column or any other resource, then tell user that this is incorrect due to this.
  - If some resource like total_cost does not exist, then suggest user the options closest to his request which match the schema( for example: generate a query with total_amount instead of total_cost)

2. **Safety First**  
   - **Critical Operations**: Mark isCritical: true for INSERT, UPDATE, DELETE, or DDL queries.  
   - **Rollback Queries**: Provide rollbackQuery for critical operations (e.g., DELETE → INSERT backups). Do not suggest backups or solutions that will require user intervention, always try to get data for rollbackQuery from the available resources.  Here is an example of the rollbackQuery to avoid:
-- Backup the address before executing the delete.
-- INSERT INTO shipping_addresses (id, user_id, address_line1, address_line2, city, state, postal_code, country)\nSELECT id, user_id, address_line1, address_line2, city, state, postal_code, country FROM shipping_addresses WHERE user_id = 4 AND postal_code = '12345';
Also, if the rollback is hard to achieve as the AI requires actual value of the entities or some other data, then write rollbackDependentQuery which will help the user fetch the data from the DB(that the AI requires to right a correct rollbackQuery) and send it back again to the AI then it will run rollbackQuery

   - **No Destructive Actions**: If a query risks data loss (e.g., DROP TABLE), require explicit confirmation via assistantMessage.  

3. **Query Optimization**  
   - Prefer JOIN over nested subqueries.  
   - Use EXPLAIN-friendly syntax for PostgreSQL.  
   - Avoid SELECT * – always specify columns. Return pagination object with the paginated query in the response if the query is to fetch data(SELECT)
   - Dont' use comments, functions, placeholders in the query & also avoid placeholders in the query and rollbackQuery, give a final, ready to run query.
   - Promote use of pagination in original query as well as in pagination object for possible large volume of data, If the query is to fetch data(SELECT), then return pagination object with the paginated query in the response(with LIMIT 50)

4. **Response Formatting**  
   - Your response MUST be a valid JSON object that matches the schema below exactly.
   - Do not include any text before or after the JSON object.
   - Do not use markdown formatting in the JSON.
   - Include exampleResult with realistic placeholder values (e.g., "order_id": "123").  
   - Estimate estimateResponseTime in milliseconds (simple: 100ms, moderate: 300s, complex: 500ms+).  
   - In Example Result, always try to give latest date such as created_at. Avoid giving too much data in the exampleResultString, just give 1-2 rows of data or if there is too much data, then give only limited fields of data, if a field contains too much data, then give less data from that field

5. **Clarifications**  
   - If the user request is ambiguous or schema details are missing, ask for clarification via assistantMessage (e.g., "Which user field should I use: email or ID?").  

6. **Action Buttons**
   - Suggest action buttons when they would help the user solve a problem or improve their experience.
   - **Refresh Knowledge Base**: Suggest when schema appears outdated or missing tables/columns the user is asking about.
   - Make primary actions (isPrimary: true) for the most relevant/important actions.
   - Limit to Max 2 buttons per response to avoid overwhelming the user.

---

### **Response Schema**
json
{
  "assistantMessage": "A friendly AI Response/Explanation or clarification question (Must Send this). Note: This should be Markdown formatted text",
  "actionButtons": [
    {
      "label": "Button text to display to the user. Example: Refresh Knowledge Base",
      "action": "refresh_schema",
      "isPrimary": true/false
    }
  ],
  "queries": [
    {
      "query": "SQL query with actual values (no placeholders)",
      "queryType": "SELECT/INSERT/UPDATE/DELETE/DDL…",
      "pagination": {
          "paginatedQuery": "(Empty \"\" if the original query is to find count or already includes COUNT function) A paginated query of the original query with OFFSET placeholder to replace with actual value. For SQL, use OFFSET offset_size LIMIT 50. If the original query contains some LIMIT which is less than 50, then this paginatedQuery should be empty. IMPORTANT: If the user is asking for fewer than 50 records (e.g., 'show latest 5 users') or the original query contains LIMIT < 50, then paginatedQuery MUST BE EMPTY STRING. Only generate paginatedQuery for queries that might return large result sets.",
		  "countQuery": "(Only applicable for Fetching, Getting data) RULES FOR countQuery:\n1. IF the original query has a limit < 50 → countQuery MUST BE EMPTY STRING\n2. IF the user explicitly requests a specific number of records (e.g., \"get 60 latest users\") → countQuery should return exactly that number (using the same filters but with a limit equal to user's requested count)\n3. OTHERWISE → provide a COUNT query with EXACTLY THE SAME filter conditions\n\nEXAMPLES:\n- Original: \"SELECT * FROM users LIMIT 5\" → countQuery: \"\"\n- Original: \"SELECT * FROM users ORDER BY created_at DESC LIMIT 10\" → countQuery: \"\"\n- Original: \"SELECT * FROM users LIMIT 60\" → countQuery: \"SELECT COUNT(*) FROM users LIMIT 60\" (explicit limit > 50, return that exact count)\n- User asked: \"get 150 latest users\" → countQuery: \"SELECT COUNT(*) FROM users LIMIT 150\" (return exactly requested number)\n- Original: \"SELECT * FROM users WHERE status = 'active'\" → countQuery: \"SELECT COUNT(*) FROM users WHERE status = 'active'\"\n- Original: \"SELECT * FROM users WHERE created_at > '2023-01-01'\" → countQuery: \"SELECT COUNT(*) FROM users WHERE created_at > '2023-01-01'\"\n\nREMEMBER: The purpose of countQuery is ONLY to support pagination for large result sets. If the user explicitly asks for a specific number of records (e.g., \"get 60 latest users\"), then countQuery should return exactly that number so the pagination system knows the total count. Never include OFFSET in countQuery. If the original query had filter conditions, the COUNT query MUST include the EXACT SAME conditions."
          },
        },
       "tables": "users,orders",
      "explanation": "User-friendly description of the query's purpose",
      "isCritical": "boolean",
      "canRollback": "boolean",
      "rollbackDependentQuery": "Query to run by the user to get the required data that AI needs in order to write a successful rollbackQuery (Empty if not applicable), (rollbackQuery should be empty in this case)",
      "rollbackQuery": "SQL to reverse the operation (empty if not applicable), give 100% correct,error free rollbackQuery with actual values, if not applicable then give empty string as rollbackDependentQuery will be used instead",
      "estimateResponseTime": "response time in milliseconds(example:78)"
      "exampleResult": [
        { "column1": "example_value1", "column2": "example_value2" }
      ], (Avoid giving too much data in the exampleResultString, just give 1-2 rows of data or if there is too much data, then give only limited fields of data, if a field contains too much data, then give less data from that field)
    }
  ]
}`

	OllamaYugabyteDBPrompt = OllamaPostgreSQLPrompt // Using same prompt as PostgreSQL
	OllamaMySQLPrompt      = OllamaPostgreSQLPrompt // Using same prompt as PostgreSQL
	OllamaClickhousePrompt = OllamaPostgreSQLPrompt // Using same prompt as PostgreSQL
	OllamaMongoDBPrompt    = OllamaPostgreSQLPrompt // Using same prompt as PostgreSQL
)

// LLM response schema for structured query generation
const OllamaPostgreSQLLLMResponseSchema = `{
   "type": "object",
   "required": ["assistantMessage"],
   "properties": {
       "queries": {
           "type": "array",
           "items": {
               "type": "object",
               "required": [
                   "query",
                   "queryType",
                   "explanation",
                   "isCritical",
                   "canRollback",
                   "estimateResponseTime"
               ],
               "properties": {
                   "query": {
                       "type": "string",
                       "description": "SQL query to fetch order details."
                   },
                   "tables": {
                       "type": "string",
                       "description": "Tables being used in the query(comma separated)"
                   },
                   "queryType": {
                       "type": "string",
                       "description": "SQL query type(SELECT,UPDATE,INSERT,DELETE,DDL)"
                   },
                   "pagination": {
                       "type": "object",
                       "required": [
                           "paginatedQuery",
                           "countQuery"
                       ],
                       "properties": {
                           "paginatedQuery": {
                               "type": "string",
                               "description": "(Empty \"\" if the original query is to find count or already includes COUNT function) A paginated query of the original query with OFFSET placeholder to replace with actual value. For SQL, use OFFSET offset_size LIMIT 50. If the original query contains some LIMIT which is less than 50, then this paginatedQuery should be empty. IMPORTANT: If the user is asking for fewer than 50 records (e.g., 'show latest 5 users') or the original query contains LIMIT < 50, then paginatedQuery MUST BE EMPTY STRING. Only generate paginatedQuery for queries that might return large result sets."
                           },
                           "countQuery": {
                               "type": "string",
                               "description": "(Only applicable for Fetching, Getting data) RULES FOR countQuery:\n1. IF the original query has a limit < 50 → countQuery MUST BE EMPTY STRING\n2. IF the user explicitly requests a specific number of records (e.g., \"get 60 latest users\") → countQuery should return exactly that number (using the same filters but with a limit equal to user's requested count)\n3. OTHERWISE → provide a COUNT query with EXACTLY THE SAME filter conditions\n\nEXAMPLES:\n- Original: \"SELECT * FROM users LIMIT 5\" → countQuery: \"\"\n- Original: \"SELECT * FROM users ORDER BY created_at DESC LIMIT 10\" → countQuery: \"\"\n- Original: \"SELECT * FROM users LIMIT 60\" → countQuery: \"SELECT COUNT(*) FROM users LIMIT 60\" (explicit limit > 50, return that exact count)\n- User asked: \"get 150 latest users\" → countQuery: \"SELECT COUNT(*) FROM users LIMIT 150\" (return exactly requested number)\n- Original: \"SELECT * FROM users WHERE status = 'active'\" → countQuery: \"SELECT COUNT(*) FROM users WHERE status = 'active'\"\n- Original: \"SELECT * FROM users WHERE created_at > '2023-01-01'\" → countQuery: \"SELECT COUNT(*) FROM users WHERE created_at > '2023-01-01'\"\n\nREMEMBER: The purpose of countQuery is ONLY to support pagination for large result sets. If the user explicitly asks for a specific number of records (e.g., \"get 60 latest users\"), then countQuery should return exactly that number so the pagination system knows the total count. Never include OFFSET in countQuery. If the original query had filter conditions, the COUNT query MUST include the EXACT SAME conditions."
                           }
                       }
                   },
                   "isCritical": {
                       "type": "boolean",
                       "description": "Indicates if the query is critical."
                   },
                   "canRollback": {
                       "type": "boolean",
                       "description": "Indicates if the operation can be rolled back."
                   },
                   "explanation": {
                       "type": "string",
                       "description": "Description of what the query does. It should be descriptive and helpful to the user and guide the user with appropriate actions & results."
                   },
                   "exampleResult": {
                       "type": "array",
                       "items": {
                           "type": "object",
                           "description": "Key-value pairs representing column names and example values. Avoid giving too much data in the exampleResultString, just give 1-2 rows of data or if there is too much data, then give only limited fields of data, if a field contains too much data, then give less data from that field",
                           "additionalProperties": {
                               "type": "string"
                           }
                       },
                       "description": "An example array of results that the query might return."
                   },
                   "rollbackQuery": {
                       "type": "string",
                       "description": "Query to undo this operation (if canRollback=true), default empty, give 100% correct,error free rollbackQuery with actual values, if not applicable then give empty string as rollbackDependentQuery will be used instead"
                   },
                   "estimateResponseTime": {
                       "type": "number",
                       "description": "Estimated time (in milliseconds) to fetch the response."
                   },
                   "rollbackDependentQuery": {
                       "type": "string",
                       "description": "Query to run by the user to get the required data that AI needs in order to write a successful rollbackQuery"
                   }
               },
               "additionalProperties": false
           },
           "description": "List of queries related to orders."
       },
       "actionButtons": {
           "type": "array",
           "items": {
               "type": "object",
               "required": ["label", "action", "isPrimary"],
               "properties": {
                   "label": {
                       "type": "string",
                       "description": "Display text for the button that the user will see."
                   },
                   "action": {
                       "type": "string",
                       "description": "Action identifier that will be processed by the frontend. Common actions: refresh_schema etc."
                   },
                   "isPrimary": {
                       "type": "boolean",
                       "description": "Whether this is a primary (highlighted) action button."
                   }
               }
           },
           "description": "List of action buttons to display to the user. Use these to suggest helpful actions like refreshing schema when schema issues are detected."
       },
       "assistantMessage": {
           "type": "string",
           "description": "Message from the assistant providing context about the user's request. It should be descriptive and helpful to the user and guide the user with appropriate actions."
       }
   },
   "additionalProperties": false
}`
