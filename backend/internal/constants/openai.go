package constants

// OpenAI Initial LLM Response Schema
const OpenAILLMResponseSchema = `{
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
                       "description": "DB query to fetch details from database."
                   },
                   "tables": {
                       "type": "string",
                       "description": "Tables/collection being used in the query(comma separated)"
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
                               "description": "(Only applicable for Fetching, Getting data) RULES FOR countQuery:\n1. IF the original query has a limit < 50 -> countQuery MUST BE EMPTY STRING\n2. IF the user explicitly requests a specific number of records (e.g., \"get 60 latest users\") -> countQuery should return exactly that number (using the same filters but with a limit equal to user's requested count)\n3. OTHERWISE -> provide a COUNT query with EXACTLY THE SAME filter conditions\n\nEXAMPLES:\n- Original: \"SELECT * FROM users LIMIT 5\" -> countQuery: \"\"\n- Original: \"SELECT * FROM users ORDER BY created_at DESC LIMIT 10\" -> countQuery: \"\"\n- Original: \"SELECT * FROM users LIMIT 60\" -> countQuery: \"SELECT COUNT(*) FROM users LIMIT 60\" (explicit limit > 50, return that exact count)\n- User asked: \"get 150 latest users\" -> countQuery: \"SELECT COUNT(*) FROM users LIMIT 150\" (return exactly requested number)\n- Original: \"SELECT * FROM users WHERE status = 'active'\" -> countQuery: \"SELECT COUNT(*) FROM users WHERE status = 'active'\"\n- Original: \"SELECT * FROM users WHERE created_at > '2023-01-01'\" -> countQuery: \"SELECT COUNT(*) FROM users WHERE created_at > '2023-01-01'\"\n\nREMEMBER: The purpose of countQuery is ONLY to support pagination for large result sets. If the user explicitly asks for a specific number of records (e.g., \"get 60 latest users\"), then countQuery should return exactly that number so the pagination system knows the total count. Never include OFFSET in countQuery. If the original query had filter conditions, the COUNT query MUST include the EXACT SAME conditions."
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

const OpenAIRecommendationsResponseSchema = `{
  "type": "object",
  "properties": {
    "recommendations": {
      "type": "array",
      "description": "An array of 60 query recommendations (minimum 40 if 60 not possible)",
      "items": {
        "type": "object",
        "required": ["text"],
        "properties": {
          "text": {
            "type": "string",
            "description": "The recommendation text that users can ask"
          }
        }
      }
    }
  },
  "required": ["recommendations"]
}`
