package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"neobase-ai/internal/models"
	"neobase-ai/pkg/dbmanager"
	"neobase-ai/pkg/llm"
	"strings"
)

const paginatedQueryFixMaxIterations = 5

// getPaginatedQueryFixerTools returns the two tools used by the auto-fixer:
//   - test_paginated_query: lets the LLM test a candidate template (with {{cursor_value}})
//     against the real DB using a sample cursor value
//   - generate_final_response (FinalResponseToolName): submit the confirmed fixed *template*
func getPaginatedQueryFixerTools() []llm.ToolDefinition {
	return []llm.ToolDefinition{
		{
			Name: "test_paginated_query",
			Description: "Execute the proposed fixed paginatedQuery template against the database. " +
				"The template MUST still contain {{cursor_value}} — the system will substitute the " +
				"sample cursor value automatically before running the test.",
			Parameters: map[string]interface{}{
				"type":     "object",
				"required": []string{"query"},
				"properties": map[string]interface{}{
					"query": map[string]interface{}{
						"type":        "string",
						"description": "The proposed fixed paginatedQuery template (must still contain {{cursor_value}}).",
					},
				},
			},
		},
		{
			Name:        llm.FinalResponseToolName,
			Description: "Submit the fixed paginatedQuery template after test_paginated_query has confirmed it executes successfully. The template must still contain {{cursor_value}}.",
			Parameters: map[string]interface{}{
				"type":     "object",
				"required": []string{"fixed_query"},
				"properties": map[string]interface{}{
					"fixed_query": map[string]interface{}{
						"type":        "string",
						"description": "The fixed paginatedQuery template (with {{cursor_value}} placeholder intact).",
					},
				},
			},
		},
	}
}

// autoFixPaginatedQuery invokes the LLM in a lightweight tool-calling loop to repair a
// broken paginatedQuery *template*. The LLM receives the original template (which still
// contains {{cursor_value}}) and the error that occurred. It proposes a fixed template,
// validates it via test_paginated_query (where the executor substitutes the sample cursor
// before testing), then calls generate_final_response with the fixed template.
//
// The returned fixed query is always the *template* — {{cursor_value}} is preserved —
// so it is safe to persist and reuse for every subsequent pagination call.
func autoFixPaginatedQuery(
	ctx context.Context,
	llmClient llm.Client,
	dbManager *dbmanager.Manager,
	chatID string,
	dbType string,
	queryType string, // e.g. "SELECT"
	originalTemplate string, // paginatedQuery template — contains {{cursor_value}} placeholder
	sampleCursor string, // the cursor value that was in use when the error occurred (for test)
	queryError string,
) (string, error) {
	if llmClient == nil {
		return "", fmt.Errorf("no LLM client available for paginatedQuery auto-fix")
	}

	tools := getPaginatedQueryFixerTools()

	// Executor: only handles test_paginated_query.
	// The proposed template still has {{cursor_value}}, so we substitute the sample cursor
	// before executing — this validates the template produces a working query.
	executor := func(ctx context.Context, call llm.ToolCall) (*llm.ToolResult, error) {
		if call.Name != "test_paginated_query" {
			return &llm.ToolResult{CallID: call.ID, Name: call.Name, Content: "unknown tool", IsError: true}, nil
		}
		template, _ := call.Arguments["query"].(string)
		if template == "" {
			return &llm.ToolResult{CallID: call.ID, Name: call.Name, Content: "error: query argument is empty", IsError: true}, nil
		}
		// Substitute the sample cursor into the template for testing.
		// The template should keep {{cursor_value}} — replace it here with the real value.
		testQuery := strings.ReplaceAll(template, "{{cursor_value}}", sampleCursor)
		_, execErr := dbManager.ExecuteQuery(ctx, chatID, "", "", "", testQuery, queryType, false, false)
		if execErr != nil {
			return &llm.ToolResult{
				CallID:  call.ID,
				Name:    call.Name,
				Content: fmt.Sprintf("Query failed: %s", execErr.Message),
				IsError: true,
			}, nil
		}
		return &llm.ToolResult{CallID: call.ID, Name: call.Name, Content: "Query executed successfully — no error."}, nil
	}

	userMessage := fmt.Sprintf(
		"A cursor-based paginatedQuery TEMPLATE failed. Fix the template so it executes without error.\n\n"+
			"DB Type: %s\n\n"+
			"Original paginatedQuery template (contains {{cursor_value}} placeholder):\n%s\n\n"+
			"Sample cursor value that was substituted when the error occurred: %s\n\n"+
			"Error:\n%s\n\n"+
			"Instructions:\n"+
			"1. Identify the syntax problem in the template.\n"+
			"2. Propose a corrected template. CRITICAL: the fixed template MUST still contain the\n"+
			"   {{cursor_value}} placeholder exactly as-is — do NOT replace it with any actual value.\n"+
			"3. Call test_paginated_query with your proposed template — the system will automatically\n"+
			"   substitute the sample cursor value for testing.\n"+
			"4. If it fails, iterate. Once test_paginated_query succeeds, call generate_final_response\n"+
			"   with ONLY: {\"fixed_query\": \"<template still containing {{cursor_value}}>\"}.",
		dbType, originalTemplate, sampleCursor, queryError,
	)

	messages := []*models.LLMMessage{
		{
			Role: "user",
			Content: map[string]interface{}{
				"user_message": userMessage,
			},
		},
	}

	config := llm.ToolCallConfig{
		MaxIterations: paginatedQueryFixMaxIterations,
		DBType:        dbType,
		SystemPrompt: "IMPORTANT: For this request your ONLY task is to fix the broken paginatedQuery template. " +
			"The fixed template MUST preserve the {{cursor_value}} placeholder — never substitute a real value. " +
			"Call test_paginated_query first to validate your fix. " +
			"Once it succeeds, call generate_final_response with ONLY the field \"fixed_query\" containing " +
			"the fixed template — no assistantMessage, no queries array, no other fields.",
	}

	result, err := llmClient.GenerateWithTools(ctx, messages, tools, executor, config)
	if err != nil {
		return "", fmt.Errorf("LLM paginatedQuery auto-fix error: %w", err)
	}

	// Extract fixed_query from generate_final_response arguments
	var fixedResp struct {
		FixedQuery string `json:"fixed_query"`
	}
	if err := json.Unmarshal([]byte(result.Response), &fixedResp); err != nil {
		log.Printf("[QUERY_FIX] Could not parse fix response: %v — raw: %.200s", err, result.Response)
		return "", fmt.Errorf("failed to parse fix response: %w", err)
	}
	if fixedResp.FixedQuery == "" {
		return "", fmt.Errorf("LLM returned empty fixed_query")
	}
	// Safety check: if the AI accidentally returned a concrete query with no placeholder,
	// reject it — persisting it would break all future pagination.
	if !strings.Contains(fixedResp.FixedQuery, "{{cursor_value}}") {
		log.Printf("[QUERY_FIX] WARNING: AI-returned fixed_query has no {{cursor_value}} placeholder — discarding. raw: %.200s", fixedResp.FixedQuery)
		return "", fmt.Errorf("fixed_query is missing {{cursor_value}} placeholder")
	}

	log.Printf("[QUERY_FIX] paginatedQuery auto-fixed in %d iterations (%d tool calls)", result.Iterations, result.TotalCalls)
	return fixedResp.FixedQuery, nil
}
