import { ActionButton, Message } from "../components/chat/types";


// Update MessagesResponse to use BackendMessage instead of Message
export interface MessagesResponse {
    success: boolean;
    data: {
        messages: BackendMessage[];
        total: number;
    };
}

// Add interface for backend message format
export interface BackendMessage {
    id: string;
    chat_id: string;
    type: 'user' | 'assistant';
    content: string;
    user_message_id?: string;
    is_edited: boolean;
    non_tech_mode?: boolean;
    is_pinned?: boolean;
    pinned_at?: string;
    action_buttons?: ActionButton[];
    queries?: {
        id: string;
        query: string;
        is_edited: boolean;
        description: string;
        execution_time: number;
        example_execution_time: number;
        can_rollback: boolean;
        is_critical: boolean;
        is_executed: boolean;
        is_rolled_back: boolean;
        error?: {
            code: string;
            message: string;
            details?: string;
        };
        example_result: any[];
        execution_result: any[];
        action_at?: string;
        query_type: string;
        pagination?: {
            total_records_count?: number;
            paginated_query?: string;
        };
        tables: string;
        rollback_query?: string;
        rollback_dependent_query?: string;
    }[];
    created_at: string;
}

// Add transform function
export const transformBackendMessage = (msg: BackendMessage): Message => {
    // console.log('transformBackendMessage -> msg', msg);
    return {
        id: msg.id,
        type: msg.type,
        content: msg.content,
        queries: msg.queries || [],
        user_message_id: msg.user_message_id,
        is_loading: false,
        loading_steps: [],
        is_streaming: false,
        is_edited: msg.is_edited,
        created_at: msg.created_at,
        action_buttons: msg.action_buttons || [],
        non_tech_mode: msg.non_tech_mode,
        is_pinned: msg.is_pinned,
        pinned_at: msg.pinned_at
    };
};

// Add interface for the API response
export interface SendMessageResponse {
    success: boolean;
    data: {
        id: string;
        chat_id: string;
        type: 'user' | 'assistant';
        content: string;
        created_at: string;
    };
}

export interface ExecuteQueryResponse {
    success: boolean;
    data: {
        chat_id: string;
        message_id: string;
        query_id: string;
        execution_time?: number;
        execution_result?: any[];
        total_records_count: number;
        is_rolled_back: boolean;
        is_executed: boolean;
        error?: {
            code: string;
            message: string;
            details?: string;
        };
        action_buttons?: ActionButton[];
        action_at?: string;
    };
}