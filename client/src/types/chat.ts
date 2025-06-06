// Create a new file for chat types
export type SSLMode = 'disable' | 'require' | 'verify-ca' | 'verify-full';

export interface Connection {
    type: 'postgresql' | 'yugabytedb' | 'mysql' | 'clickhouse' | 'mongodb' | 'redis' | 'neo4j';
    host: string;
    port: string;
    username: string;
    password?: string;
    database: string;
    auth_database?: string; // Database to authenticate against (for MongoDB)
    is_example_db: boolean;
    use_ssl?: boolean;
    ssl_mode?: SSLMode;
    ssl_cert_url?: string;
    ssl_key_url?: string;
    ssl_root_cert_url?: string;
    // SSH tunnel fields
    ssh_enabled?: boolean;
    ssh_host?: string;
    ssh_port?: string;
    ssh_username?: string;
    ssh_private_key?: string;
    ssh_passphrase?: string;
}

export interface Chat {
    id: string;
    user_id: string;
    connection: Connection;
    selected_collections?: string; // "ALL" or comma-separated table names
    settings: ChatSettings;
    created_at: string;
    updated_at: string;
}

export interface ChatsResponse {
    success: boolean;
    data: {
        chats: Chat[];
        total: number;
    };
}

export interface CreateChatResponse {
    success: boolean;
    data: Chat;
}

// Table and column information
export interface ColumnInfo {
    name: string;
    type: string;
    is_nullable: boolean;
}

export interface TableInfo {
    name: string;
    columns: ColumnInfo[];
    is_selected: boolean;
}

export interface TablesResponse {
    tables: TableInfo[];
} 

export interface ChatSettings {
    auto_execute_query: boolean;
    share_data_with_ai: boolean;
}