package constants

import "fmt"

// QueryRetryPromptTemplate is the user-facing prompt template sent to the LLM when a query execution fails
// with a fixable error (syntax errors, operator issues, type mismatches, etc.).
// It instructs the LLM to analyze the failed query and the database error, then return a corrected
// query using the standard structured response schema (queries array).
// Parameters: dbType, failedQuery, errorMessage
const QueryRetryPromptTemplate = `A query was just executed against the connected %s database but it failed with an error. 
Your task is to analyze the failed query and the error message, then provide a corrected version of the query that will execute successfully.

**Failed Query:**
%s

**Error Message:**
%s

**Instructions:**
- Fix the query based on the error message. Common issues include: syntax errors, wrong column/table names, incorrect operators, type mismatches, missing quotes, or invalid aggregation pipeline stages.
- Return the corrected query using the standard response format (in the queries array).
- Keep the query's original intent — only fix what's broken.
- If the query uses database-specific syntax (e.g., MongoDB aggregation), ensure the corrected version is valid for that database type.
- Set isCritical, canRollback, and other fields appropriately for the corrected query.
- In the assistantMessage, briefly explain what was wrong and what you fixed.`

// GetQueryRetryPrompt returns the formatted retry prompt with the given parameters.
func GetQueryRetryPrompt(dbType, failedQuery, errorMessage string) string {
	return fmt.Sprintf(QueryRetryPromptTemplate, dbType, failedQuery, errorMessage)
}

// StructuralErrorPromptTemplate is used when a query fails with a non-retryable/structural error
// (e.g., table/collection doesn't exist, permission denied, authentication failure).
// Instead of retrying the query, the LLM should explain the issue clearly to the user.
// Parameters: dbType, failedQuery, errorMessage
const StructuralErrorPromptTemplate = `A query was just executed against the connected %s database but it failed with a structural error that cannot be fixed by modifying the query itself.

**Failed Query:**
%s

**Error Message:**
%s

**Instructions:**
- Do NOT return any queries in the response. Leave the queries array empty [].
- In the assistantMessage, explain the error clearly in simple, user-friendly language.
- If the error indicates a table/collection doesn't exist, tell the user which table/collection was not found and suggest they check the available tables/collections in their database or refresh the schema.
- If the error indicates permission/authentication issues, explain that and suggest checking database credentials or permissions.
- Be concise but helpful. Use plain language, not technical jargon.
- You may suggest action buttons like "Refresh Schema" (action: "refresh_schema") if appropriate.`

// GetStructuralErrorPrompt returns the formatted structural error prompt.
func GetStructuralErrorPrompt(dbType, failedQuery, errorMessage string) string {
	return fmt.Sprintf(StructuralErrorPromptTemplate, dbType, failedQuery, errorMessage)
}
