// Create a new file for chat types
export type SSLMode = 'disable' | 'require' | 'verify-ca' | 'verify-full';

export interface FileUpload {
    id: string;
    filename: string;
    size: number;
    type: 'csv' | 'excel';
    uploadedAt: Date;
    tableName?: string; // The name of the table this file will be imported as
    sheetNames?: string[]; // For Excel files with multiple sheets
    file?: File; // The actual File object for upload
    mergeStrategy?: 'replace' | 'append' | 'merge' | 'smart_merge'; // How to handle conflicts with existing tables
    mergeOptions?: {
        keyColumns?: string[]; // Columns to use as keys for matching rows
        ignoreCase?: boolean; // Ignore case when comparing values
        trimWhitespace?: boolean; // Trim whitespace from values
        handleNulls?: 'keep' | 'empty' | 'null'; // How to handle null values
        addNewColumns?: boolean; // Add columns that exist in new data
        dropMissingColumns?: boolean; // Drop columns not in new data
        updateExisting?: boolean; // Update existing rows
        insertNew?: boolean; // Insert new rows
        deleteMissing?: boolean; // Delete rows not in new data
    };
}

export interface Connection {
    type: 'postgresql' | 'yugabytedb' | 'mysql' | 'clickhouse' | 'mongodb' | 'redis' | 'neo4j' | 'spreadsheet' | 'google_sheets';
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
    // Spreadsheet specific fields
    file_uploads?: FileUpload[];
    schema_name?: string; // Schema name in the CSV PostgreSQL database
    // Google Sheets specific fields
    google_sheet_id?: string;
    google_sheet_url?: string;
    google_auth_token?: string;
    google_refresh_token?: string;
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
    non_tech_mode: boolean;
}