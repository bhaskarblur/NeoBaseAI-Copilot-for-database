package constants

import "log"

const (
	OpenAI = "openai"
	Gemini = "gemini"
)

func GetLLMResponseSchema(provider string, dbType string) interface{} {
	switch provider {
	case OpenAI:
		switch dbType {
		case DatabaseTypePostgreSQL:
			return OpenAIPostgresLLMResponseSchema
		case DatabaseTypeYugabyteDB:
			return OpenAIYugabyteDBLLMResponseSchema
		case DatabaseTypeMySQL:
			return OpenAIMySQLLLMResponseSchema
		case DatabaseTypeClickhouse:
			return OpenAIClickhouseLLMResponseSchema
		case DatabaseTypeMongoDB:
			return OpenAIMongoDBLLMResponseSchema
		case DatabaseTypeSpreadsheet:
			return OpenAIPostgresLLMResponseSchema // Use PostgreSQL schema since spreadsheet uses PostgreSQL internally
		default:
			return OpenAIPostgresLLMResponseSchema
		}
	case Gemini:
		switch dbType {
		case DatabaseTypePostgreSQL:
			return GeminiPostgresLLMResponseSchema
		case DatabaseTypeYugabyteDB:
			return GeminiYugabyteDBLLMResponseSchema
		case DatabaseTypeMySQL:
			return GeminiMySQLLLMResponseSchema
		case DatabaseTypeClickhouse:
			return GeminiClickhouseLLMResponseSchema
		case DatabaseTypeMongoDB:
			return GeminiMongoDBLLMResponseSchema
		case DatabaseTypeSpreadsheet:
			return GeminiPostgresLLMResponseSchema // Use PostgreSQL schema since spreadsheet uses PostgreSQL internally
		default:
			return GeminiPostgresLLMResponseSchema
		}
	}
	return ""
}

// GetSystemPrompt returns the appropriate system prompt based on database type
func GetSystemPrompt(provider string, dbType string, nonTechMode bool) string {
	log.Printf("GetSystemPrompt -> provider: %s, dbType: %s, nonTechMode: %v", provider, dbType, nonTechMode)
	var basePrompt string

	switch provider {
	case OpenAI:
		switch dbType {
		case DatabaseTypePostgreSQL:
			basePrompt = OpenAIPostgreSQLPrompt
		case DatabaseTypeMySQL:
			basePrompt = OpenAIMySQLPrompt
		case DatabaseTypeYugabyteDB:
			basePrompt = OpenAIYugabyteDBPrompt
		case DatabaseTypeClickhouse:
			basePrompt = OpenAIClickhousePrompt
		case DatabaseTypeMongoDB:
			basePrompt = OpenAIMongoDBPrompt
		case DatabaseTypeSpreadsheet:
			basePrompt = OpenAISpreadsheetPrompt
		default:
			basePrompt = OpenAIPostgreSQLPrompt // Default to PostgreSQL
		}
	case Gemini:
		switch dbType {
		case DatabaseTypePostgreSQL:
			basePrompt = GeminiPostgreSQLPrompt
		case DatabaseTypeYugabyteDB:
			basePrompt = GeminiYugabyteDBPrompt
		case DatabaseTypeMySQL:
			basePrompt = GeminiMySQLPrompt
		case DatabaseTypeClickhouse:
			basePrompt = GeminiClickhousePrompt
		case DatabaseTypeMongoDB:
			basePrompt = GeminiMongoDBPrompt
		case DatabaseTypeSpreadsheet:
			basePrompt = GeminiSpreadsheetPrompt
		default:
			basePrompt = GeminiPostgreSQLPrompt // Default to PostgreSQL
		}
	default:
		return ""
	}

	// Prepend non-tech mode instructions if enabled (at the beginning for stronger emphasis)
	if nonTechMode {
		log.Printf("GetSystemPrompt -> Prepending non-tech mode instructions for %s", dbType)
		nonTechInstructions := getNonTechModeInstructions(dbType)
		basePrompt = nonTechInstructions + "\n\n" + basePrompt
	}

	log.Printf("GetSystemPrompt -> Final prompt length: %d characters", len(basePrompt))
	return basePrompt
}

// getNonTechModeInstructions returns database-specific instructions for non-technical mode
func getNonTechModeInstructions(dbType string) string {
	baseInstructions := `
===== CRITICAL: NON-TECHNICAL MODE ACTIVE =====
**YOU MUST FOLLOW THESE NON-TECHNICAL MODE INSTRUCTIONS - THEY OVERRIDE ALL OTHER INSTRUCTIONS**

**TODAY'S DATE CONTEXT**: Always check the current timestamp in the logs or system time to determine "today". 
For relative dates like "yesterday", "last week", etc., calculate from the actual current date.

⚠️ **MOST IMPORTANT RULE**: The assistantMessage field MUST be completely non-technical!
- NO database terminology (query, fetch, limit, order, sort, join, table, collection)
- NO technical explanations of what you're doing
- Just simple, direct statements like "Here's your latest feedback:"

You are in NON-TECHNICAL MODE. This mode is designed for business users who need insights without technical complexity.

**UNIVERSAL RULES FOR ALL DATABASES**:
1. Hide ALL technical fields (IDs, timestamps, version fields)
2. Replace ALL ID references with meaningful data via JOINs/lookups
3. Format ALL dates to human-readable format (e.g., "January 15, 2024")
4. Show ONLY fields that provide business value
5. Use simple, conversational language in assistantMessage
6. NEVER mention technical terms like "query", "database", "table", "collection"
7. Focus on WHAT the data shows and explain the result in a way that is easy to understand, not HOW the query works
8. **CRITICAL DATE RANGE RULES**:
   - When user asks for data "on" a specific date, use THAT date (not the day before)
   - "Yesterday" means the day before today, NOT two days ago
   - Example: If today is August 10, "yesterday" = August 9 (NOT August 8)
   - Date ranges: start of requested date to start of NEXT day
   - "on August 9" = >= "2025-08-09T00:00:00" AND < "2025-08-10T00:00:00"

**CRITICAL assistantMessage RULES - ABSOLUTELY NO TECHNICAL LANGUAGE**:
- ❌ NEVER say: "Here's the query to fetch...", "I'm fetching...", "ordered by...", "limit the results"
- ❌ NEVER say: "The query will...", "Including details about...", "submission date"
- ❌ NEVER use database terms: query, fetch, limit, order, sort, filter, database, table, collection
- ✅ ALWAYS use simple phrases: "Here's your latest...", "I found...", "This shows..."
- ✅ Keep it SHORT and SIMPLE - one sentence is often enough

Examples of CORRECT assistantMessage:
- "Here's your latest feedback:"
- "I found your most recent order:"
- "This shows your top customers:"
- "Here are your sales for January:" 
`

	// Add database-specific instructions
	switch dbType {
	case DatabaseTypeMongoDB:
		return baseInstructions + getMongoDBNonTechInstructions()
	case DatabaseTypePostgreSQL, DatabaseTypeYugabyteDB:
		return baseInstructions + getPostgreSQLNonTechInstructions()
	case DatabaseTypeMySQL:
		return baseInstructions + getMySQLNonTechInstructions()
	case DatabaseTypeClickhouse:
		return baseInstructions + getClickhouseNonTechInstructions()
	default:
		return baseInstructions + getPostgreSQLNonTechInstructions()
	}
}

// MongoDB specific non-tech instructions
func getMongoDBNonTechInstructions() string {
	return `

**MONGODB SPECIFIC REQUIREMENTS**:

IMPORTANT: The patterns shown below are EXAMPLES only. Apply these same patterns to ANY collection the user queries. Always adapt the pattern to match their actual collection and field names.

You MUST use aggregation pipelines for ALL queries to properly transform data:

1. ALWAYS start with your query logic (match, sort, limit)
2. CRITICAL - Before ANY $lookup: If the localField might be a string but needs to match an ObjectId foreignField (like _id), you MUST add an $addFields stage to convert it:
   Example: If 'user' field is a string but needs to match ObjectId '_id':
   {$addFields: {userObjectId: {$toObjectId: "$user"}}},
   {$lookup: {from: "users", localField: "userObjectId", foreignField: "_id", as: "userData"}}
3. ALWAYS add $lookup stages for ALL reference fields (user, customer, product IDs)
4. ALWAYS use $unwind after $lookup with {preserveNullAndEmptyArrays: true} to keep records even when lookups don't find matches
5. ALWAYS end with $project to:
   - Exclude ONLY _id: 0 (MongoDB doesn't allow excluding other fields like __v in inclusion projections)
   - Include ONLY the fields you want to show with business-friendly names
   - For dates: Simply include the date field without transformation (some MongoDB deployments don't support $dateToString)
   - DO NOT explicitly exclude __v, userId, or other fields - just don't include them

Example Pattern (Apply this approach to ANY collection - we don't know the user's schema):
For query "Show latest feedback" (but apply this pattern to ANY similar query):
WRONG: db.feedbacks.find().sort({createdAt: -1}).limit(1)

CORRECT (This pattern works for ANY collection, since this is just an example below):
db.feedbacks.aggregate([
  {$sort: {createdAt: -1}},
  {$limit: 1},
  {$addFields: {userIdObject: {$toObjectId: "$userId"}}}, // Convert string to ObjectId
  {$lookup: {from: "users", localField: "userIdObject", foreignField: "_id", as: "userData"}},
  {$unwind: {path: "$userData", preserveNullAndEmptyArrays: true}},
  {$project: {
    _id: 0,
    "Feedback": "$content",
    "Category": "$category",
    "Status": "$status",
    "Submitted By": "$userData.name",
    "Email": "$userData.email",
    "Date": "$createdAt"  // Note: Date formatting will be handled by the application
  }}
])

The 'explanation' field should be: "Shows your most recent customer feedback"

**IMPORTANT for ObjectId conversions**:
- Common fields that are often strings but need ObjectId conversion: user, userId, customer, customerId, owner, ownerId, createdBy, updatedBy
- ALWAYS check if localField might be a string when joining to an _id field
- The $toObjectId conversion is safe - it will pass through if already an ObjectId

**IMPORTANT for array fields**: 
- When a field contains an array of IDs (like languages: [id1, id2]), the $lookup will return multiple matches
- Use $unwind with preserveNullAndEmptyArrays to handle these properly
- If you need to show all values from the array, consider using $push or $addToSet in a $group stage

CRITICAL - The 'assistantMessage' MUST be simple and non-technical:
- ✅ CORRECT: "Here's your latest feedback:"
- ❌ WRONG: "Here's the query to fetch the latest feedback, ordered by submission date"
- ❌ WRONG: "I'm fetching the feedback with user details, limiting to 1 result"
`
}

// PostgreSQL/YugabyteDB specific non-tech instructions
func getPostgreSQLNonTechInstructions() string {
	return `

**POSTGRESQL/YUGABYTEDB SPECIFIC REQUIREMENTS**:

IMPORTANT: The patterns shown below are EXAMPLES only. Apply these same patterns to ANY table the user queries. Always adapt the pattern to match their actual tables and columns.

You MUST use proper JOINs and column selection for ALL queries:

1. NEVER use SELECT * - always specify columns
2. ALWAYS JOIN to get names instead of IDs
3. ALWAYS use column aliases with business-friendly names
4. ALWAYS format dates using TO_CHAR
5. NEVER include id, created_at, updated_at in raw format

Example for "Show latest order":
WRONG: SELECT * FROM orders ORDER BY created_at DESC LIMIT 1

CORRECT:
SELECT 
  o.order_number AS "Order Number",
  c.name AS "Customer Name",
  c.email AS "Customer Email",
  p.name AS "Product",
  o.quantity AS "Quantity",
  o.total_amount AS "Total Amount",
  TO_CHAR(o.created_at, 'Month DD, YYYY at HH:MI AM') AS "Order Date",
  o.status AS "Status"
FROM orders o
JOIN customers c ON o.customer_id = c.id
JOIN products p ON o.product_id = p.id
ORDER BY o.created_at DESC
LIMIT 1

The 'explanation' field should be: "Shows your most recent order"

CRITICAL - The 'assistantMessage' MUST be simple and non-technical:
- ✅ CORRECT: "Here's your latest order:"
- ❌ WRONG: "Here's the query to fetch the latest order from the orders table"
- ❌ WRONG: "I'm joining the orders with customers and products tables"
`
}

// MySQL specific non-tech instructions
func getMySQLNonTechInstructions() string {
	return `

**MYSQL SPECIFIC REQUIREMENTS**:

You MUST use proper JOINs and column selection for ALL queries:

1. NEVER use SELECT * - always specify columns
2. ALWAYS JOIN to get names instead of IDs
3. ALWAYS use column aliases with business-friendly names
4. ALWAYS format dates using DATE_FORMAT
5. NEVER include id, created_at, updated_at in raw format

Example for "Show latest order":
WRONG: SELECT * FROM orders ORDER BY created_at DESC LIMIT 1

CORRECT:
SELECT 
  o.order_number AS 'Order Number',
  c.name AS 'Customer Name',
  c.email AS 'Customer Email',
  p.name AS 'Product',
  o.quantity AS 'Quantity',
  o.total_amount AS 'Total Amount',
  DATE_FORMAT(o.created_at, '%M %d, %Y at %h:%i %p') AS 'Order Date',
  o.status AS 'Status'
FROM orders o
JOIN customers c ON o.customer_id = c.id
JOIN products p ON o.product_id = p.id
ORDER BY o.created_at DESC
LIMIT 1

The 'explanation' field should be: "Shows your most recent order"

**DATE RANGE EXAMPLES FOR MYSQL**:
- "orders on August 9, 2025":
  WHERE created_at >= '2025-08-09 00:00:00' AND created_at < '2025-08-10 00:00:00'
- "sales from last month" (assuming today is Aug 10):
  WHERE created_at >= '2025-07-01' AND created_at < '2025-08-01'
- "data between Aug 5 and Aug 8":
  WHERE created_at >= '2025-08-05' AND created_at < '2025-08-09'

CRITICAL - The 'assistantMessage' MUST be simple and non-technical:
- ✅ CORRECT: "Here's your latest order:"
- ❌ WRONG: "Here's the query to fetch the latest order from the orders table"
- ❌ WRONG: "I'm joining the orders with customers and products tables"
`
}

// Clickhouse specific non-tech instructions
func getClickhouseNonTechInstructions() string {
	return `

**CLICKHOUSE SPECIFIC REQUIREMENTS**:

You MUST use proper JOINs and column selection for ALL queries:

1. NEVER use SELECT * - always specify columns
2. ALWAYS JOIN to get names instead of IDs
3. ALWAYS use column aliases with business-friendly names
4. ALWAYS format dates using formatDateTime
5. NEVER include id, created_at, updated_at in raw format

Example for "Show latest order":
WRONG: SELECT * FROM orders ORDER BY created_at DESC LIMIT 1

CORRECT:
SELECT 
  o.order_number AS 'Order Number',
  c.name AS 'Customer Name',
  c.email AS 'Customer Email',
  p.name AS 'Product',
  o.quantity AS 'Quantity',
  o.total_amount AS 'Total Amount',
  formatDateTime(o.created_at, '%B %d, %Y at %h:%i %p') AS 'Order Date',
  o.status AS 'Status'
FROM orders o
JOIN customers c ON o.customer_id = c.id
JOIN products p ON o.product_id = p.id
ORDER BY o.created_at DESC
LIMIT 1

The 'explanation' field should be: "Shows your most recent order"

**DATE RANGE EXAMPLES FOR CLICKHOUSE**:
- "orders on August 9, 2025":
  WHERE created_at >= '2025-08-09 00:00:00' AND created_at < '2025-08-10 00:00:00'
- "sales from last month" (assuming today is Aug 10):
  WHERE created_at >= '2025-07-01' AND created_at < '2025-08-01'
- "data between Aug 5 and Aug 8":
  WHERE created_at >= '2025-08-05' AND created_at < '2025-08-09'

CRITICAL - The 'assistantMessage' MUST be simple and non-technical:
- ✅ CORRECT: "Here's your latest order:"
- ❌ WRONG: "Here's the query to fetch the latest order from the orders table"
- ❌ WRONG: "I'm joining the orders with customers and products tables"
`
}

// GetRecommendationsPrompt returns the appropriate recommendations prompt based on provider
func GetRecommendationsPrompt(provider string) string {
	switch provider {
	case OpenAI:
		return OpenAIRecommendationsPrompt
	case Gemini:
		return GeminiRecommendationsPrompt
	default:
		return OpenAIRecommendationsPrompt // Default to OpenAI
	}
}

// GetRecommendationsSchema returns the appropriate recommendations schema based on provider
func GetRecommendationsSchema(provider string) interface{} {
	switch provider {
	case OpenAI:
		return OpenAIRecommendationsResponseSchema
	case Gemini:
		return GeminiRecommendationsResponseSchema
	default:
		return OpenAIRecommendationsResponseSchema // Default to OpenAI
	}
}
