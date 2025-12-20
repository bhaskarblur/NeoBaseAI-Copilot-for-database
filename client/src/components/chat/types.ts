export interface QueryResult {
    id: string;
    query: string;
    pagination?: {
        total_records_count?: number;
        paginated_query?: string;
    };
    description: string;
    execution_time?: number | null;
    example_execution_time?: number | null;
    example_result?: any[] | null;
    execution_result?: any[] | null;
    is_executed?: boolean;
    is_rolled_back?: boolean;
    error?: {
        code: string;
        message: string;
        details?: string;
    };
    is_critical?: boolean;
    can_rollback?: boolean;
    rollback_query?: string;
    rollback_dependent_query?: string;
    is_streaming?: boolean;
    is_edited?: boolean;
    action_at?: string;
}

export interface ActionButton {
    id: string;
    label: string;
    action: string;
    isPrimary: boolean;
}

export interface Message {
    id: string;
    type: 'user' | 'assistant';
    content: string;
    created_at: string;
    updated_at?: string;
    is_loading?: boolean;
    loading_steps?: LoadingStep[];
    queries?: QueryResult[];
    action_buttons?: ActionButton[];
    is_edited?: boolean;
    is_streaming?: boolean;
    user_message_id?: string;
    action_at?: string;
    non_tech_mode?: boolean;
    is_pinned?: boolean;
    pinned_at?: string;
    llm_model?: string; // LLM model ID used to generate this message (e.g., "gpt-4o", "gemini-2.0-flash")
    llm_model_name?: string; // Human-readable display name for the LLM model (e.g., "GPT-4 Omni", "Gemini 2.0 Flash")
}

export interface LoadingStep {
    text: string;
    done: boolean;
}