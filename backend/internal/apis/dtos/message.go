package dtos

import (
	"encoding/json"
	"log"
	"neobase-ai/internal/models"
)

type CreateMessageRequest struct {
	StreamID string `json:"stream_id" binding:"required"`
	Content  string `json:"content" binding:"required"`
	LLMModel string `json:"llm_model,omitempty"` // Selected LLM model ID (e.g., "gpt-4o", "gemini-2.0-flash")
}

type MessageResponse struct {
	ID            string          `json:"id"`
	ChatID        string          `json:"chat_id"`
	UserMessageID *string         `json:"user_message_id,omitempty"` // Only for AI response, this is the user message id of the message that triggered the AI response
	Type          string          `json:"type"`
	Content       string          `json:"content"`
	Queries       *[]Query        `json:"queries,omitempty"`
	ActionButtons *[]ActionButton `json:"action_buttons,omitempty"` // UI action buttons suggested by the LLM
	LLMModel      *string         `json:"llm_model,omitempty"`      // LLM model ID used to generate this message (nullable for backward compatibility)
	LLMModelName  *string         `json:"llm_model_name,omitempty"` // Display name for the LLM model (e.g., "GPT-4 Omni", "Gemini 2.0 Flash")
	IsEdited      bool            `json:"is_edited"`
	NonTechMode   bool            `json:"non_tech_mode"`       // Whether this message was generated in non-tech mode
	IsPinned      bool            `json:"is_pinned"`           // Whether this message is pinned
	PinnedAt      *string         `json:"pinned_at,omitempty"` // When the message was pinned
	CreatedAt     string          `json:"created_at"`
	UpdatedAt     string          `json:"updated_at"`
}

// ActionButton represents a UI action button that can be suggested by the LLM
type ActionButton struct {
	ID        string `json:"id"`
	Label     string `json:"label"`     // Display text for the button
	Action    string `json:"action"`    // Action identifier (e.g., "refresh_schema", "show_tables")
	IsPrimary bool   `json:"isPrimary"` // Whether this is a primary (highlighted) action
}

type Query struct {
	ID                     string                 `json:"id"`
	Query                  string                 `json:"query"`
	Description            string                 `json:"description"`
	ExecutionTime          *int                   `json:"execution_time,omitempty"`
	ExampleExecutionTime   int                    `json:"example_execution_time"`
	CanRollback            bool                   `json:"can_rollback"`
	IsCritical             bool                   `json:"is_critical"`
	IsExecuted             bool                   `json:"is_executed"`
	IsRolledBack           bool                   `json:"is_rolled_back"`
	Error                  *QueryError            `json:"error,omitempty"`
	ExampleResult          []interface{}          `json:"example_result,omitempty"`
	ExecutionResult        map[string]interface{} `json:"execution_result,omitempty"`
	QueryType              *string                `json:"query_type,omitempty"`
	Tables                 *string                `json:"tables,omitempty"`
	RollbackQuery          *string                `json:"rollback_query,omitempty"`
	RollbackDependentQuery *string                `json:"rollback_dependent_query,omitempty"`
	Pagination             *Pagination            `json:"pagination,omitempty"`
	IsEdited               bool                   `json:"is_edited"`
	ActionAt               *string                `json:"action_at,omitempty"` // The timestamp when the action was taken
}

type Pagination struct {
	TotalRecordsCount int `json:"total_records_count"` // Total records count of the query
	// We do not return the paginatedQuery and countQuery in the response
}

type QueryError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details string `json:"details"`
}

type MessageListResponse struct {
	Messages []MessageResponse `json:"messages"`
	Total    int64             `json:"total"`
}

type MessageListRequest struct {
	ChatID   string `form:"chat_id" binding:"required"`
	Page     int    `form:"page" binding:"required,min=1"`
	PageSize int    `form:"page_size" binding:"required,min=1,max=100"`
}

func ToQueryDto(queries *[]models.Query) *[]Query {
	// Call the new version with no decryption function for backward compatibility
	return ToQueryDtoWithDecryption(queries, nil)
}

// ToQueryDtoWithDecryption converts model queries to DTO queries with optional decryption
func ToQueryDtoWithDecryption(queries *[]models.Query, decryptFunc func(string) string) *[]Query {
	if queries == nil {
		return nil
	}

	queriesDto := make([]Query, len(*queries))
	for i, query := range *queries {
		log.Printf("ToQueryDto -> model query: %v", query)
		var exampleResult []interface{}
		var executionResult map[string]interface{}

		log.Printf("ToQueryDto -> saved query.ExampleResult: %v", query.ExampleResult)
		if query.ExampleResult != nil {
			log.Printf("ToQueryDto -> query.ExampleResult: %v", *query.ExampleResult)
			// Decrypt if function provided
			resultStr := *query.ExampleResult
			if decryptFunc != nil {
				resultStr = decryptFunc(resultStr)
			}
			err := json.Unmarshal([]byte(resultStr), &exampleResult)
			if err != nil {
				log.Printf("ToQueryDto -> error unmarshalling exampleResult: %v", err)
				exampleResult = []interface{}{}
			}
		}

		if query.ExecutionResult != nil {
			// Decrypt if function provided
			resultStr := *query.ExecutionResult
			if decryptFunc != nil {
				resultStr = decryptFunc(resultStr)
			}
			err := json.Unmarshal([]byte(resultStr), &executionResult)
			if err != nil {
				log.Printf("ToQueryDto -> error unmarshalling executionResult: %v", err)
				// Try unmarshalling the executionResult as a []interface{}
				var executionResultArray []interface{}
				err = json.Unmarshal([]byte(resultStr), &executionResultArray)
				if err != nil {
					log.Printf("ToQueryDto -> error unmarshalling executionResult as []interface{}: %v", err)
					executionResult = map[string]interface{}{}
				}
				executionResult = map[string]interface{}{
					"results": executionResultArray,
				}
			}
		}

		var pagination *Pagination
		if query.Pagination != nil {
			totalCount := 0
			if query.Pagination.TotalRecordsCount != nil {
				totalCount = *query.Pagination.TotalRecordsCount
			}
			pagination = &Pagination{
				TotalRecordsCount: totalCount,
			}
		}
		log.Printf("ToQueryDto -> final exampleResult: %v", exampleResult)
		queriesDto[i] = Query{
			ID:                     query.ID.Hex(),
			Query:                  query.Query,
			Description:            query.Description,
			ExecutionTime:          query.ExecutionTime,
			ExampleExecutionTime:   query.ExampleExecutionTime,
			CanRollback:            query.CanRollback,
			IsCritical:             query.IsCritical,
			IsExecuted:             query.IsExecuted,
			IsRolledBack:           query.IsRolledBack,
			Error:                  (*QueryError)(query.Error),
			ExampleResult:          exampleResult,
			ExecutionResult:        executionResult,
			QueryType:              query.QueryType,
			Tables:                 query.Tables,
			RollbackQuery:          query.RollbackQuery,
			RollbackDependentQuery: query.RollbackDependentQuery,
			Pagination:             pagination,
			IsEdited:               query.IsEdited,
			ActionAt:               query.ActionAt,
		}
	}
	return &queriesDto
}

// ToActionButtonDto converts model action buttons to DTO action buttons
func ToActionButtonDto(actionButtons *[]models.ActionButton) *[]ActionButton {
	log.Printf("ToActionButtonDto -> input actionButtons: %+v", actionButtons)
	if actionButtons == nil {
		return nil
	}

	actionButtonsDto := make([]ActionButton, len(*actionButtons))
	for i, button := range *actionButtons {
		actionButtonsDto[i] = ActionButton{
			ID:        button.ID.Hex(),
			Label:     button.Label,
			Action:    button.Action,
			IsPrimary: button.IsPrimary,
		}
	}
	log.Printf("ToActionButtonDto -> returning actionButtonsDto: %+v", actionButtonsDto)
	return &actionButtonsDto
}
