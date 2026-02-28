// Knowledge Base types matching backend DTOs

export interface FieldDescription {
  field_name: string;
  description: string;
}

export interface TableDescription {
  table_name: string;
  description: string;
  field_descriptions: FieldDescription[];
}

export interface KnowledgeBaseResponse {
  chat_id: string;
  table_descriptions: TableDescription[];
  created_at: string;
  updated_at: string;
}

export interface UpdateKnowledgeBaseRequest {
  table_descriptions: TableDescription[];
}
