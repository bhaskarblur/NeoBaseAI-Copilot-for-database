import { AlertCircle, CheckCircle, ChevronDown, Database, KeyRound, Loader2, Monitor, Settings, Table, X } from 'lucide-react';
import React, { useEffect, useRef, useState } from 'react';
import { Chat, ChatSettings, Connection, TableInfo, FileUpload } from '../../types/chat';
import chatService from '../../services/chatService';
import { BasicConnectionTab, SchemaTab, SettingsTab, SSHConnectionTab, FileUploadTab, DataStructureTab } from './components';
import { useStream } from '../../contexts/StreamContext';

// Connection tab type
type ConnectionType = 'basic' | 'ssh';

// Modal tab type
export type ModalTab = 'connection' | 'schema' | 'settings';

interface ConnectionModalProps {
  initialData?: Chat;
  initialTab?: ModalTab;
  onClose: (updatedChat?: Chat) => void;
  onEdit?: (data?: Connection, settings?: ChatSettings) => Promise<{ success: boolean, error?: string, updatedChat?: Chat }>;
  onSubmit: (data: Connection, settings: ChatSettings) => Promise<{ 
    success: boolean;
    error?: string;
    chatId?: string;
    selectedCollections?: string;
  }>;
  onUpdateSelectedCollections?: (chatId: string, selectedCollections: string) => Promise<void>;
}

export interface FormErrors {
  host?: string;
  port?: string;
  database?: string;
  username?: string;
  ssl_cert_url?: string;
  ssl_key_url?: string;
  ssl_root_cert_url?: string;
  ssh_host?: string;
  ssh_port?: string;
  ssh_username?: string;
  ssh_private_key?: string;
}

export default function ConnectionModal({ 
  initialData, 
  initialTab,
  onClose, 
  onEdit, 
  onSubmit,
  onUpdateSelectedCollections,
}: ConnectionModalProps) {
  const { generateStreamId } = useStream();
  
  // Modal tab state to toggle between Connection, Schema, and Settings
  const [activeTab, setActiveTab] = useState<ModalTab>(initialTab || 'connection');
  
  // Connection type state to toggle between basic and SSH tabs (within Connection tab)
  const [connectionType, setConnectionType] = useState<ConnectionType>('basic');
  const [fileUploads, setFileUploads] = useState<FileUpload[]>([]);
  
  // Track previous connection type to handle state persistence
  const [prevConnectionType, setPrevConnectionType] = useState<ConnectionType>('basic');
  
  // Schema tab states
  const [isLoadingTables, setIsLoadingTables] = useState(false);
  const [tables, setTables] = useState<TableInfo[]>([]);
  const [selectedTables, setSelectedTables] = useState<string[]>([]);
  const [expandedTables, setExpandedTables] = useState<Record<string, boolean>>({});
  const [schemaSearchQuery, setSchemaSearchQuery] = useState('');
  const [selectAllTables, setSelectAllTables] = useState(true);
  
  // State for handling new connections
  const [showingNewlyCreatedSchema, setShowingNewlyCreatedSchema] = useState(false);
  const [newChatId, setNewChatId] = useState<string | undefined>(undefined);
  const [currentChatData, setCurrentChatData] = useState<Chat | undefined>(initialData);
  
  // Success message state
  const [successMessage, setSuccessMessage] = useState<string | null>(null);
  
  // Form states
  const [isLoading, setIsLoading] = useState(false);
  const [formData, setFormData] = useState<Connection>({
    type: initialData?.connection.type || 'postgresql',
    host: initialData?.connection.host || '',
    port: initialData?.connection.port || '',
    username: initialData?.connection.username || '',
    password: '',  // Password is never sent back from server
    database: initialData?.connection.database || (initialData?.connection.type === 'spreadsheet' ? 'spreadsheet_db' : ''),
    auth_database: initialData?.connection.auth_database || 'admin',  // Default to 'admin' for MongoDB
    use_ssl: initialData?.connection.use_ssl || false,
    ssl_mode: initialData?.connection.ssl_mode || 'disable',
    ssl_cert_url: initialData?.connection.ssl_cert_url || '',
    ssl_key_url: initialData?.connection.ssl_key_url || '',
    ssl_root_cert_url: initialData?.connection.ssl_root_cert_url || '',
    ssh_enabled: initialData?.connection.ssh_enabled || false,
    ssh_host: initialData?.connection.ssh_host || '',
    ssh_port: initialData?.connection.ssh_port || '22',
    ssh_username: initialData?.connection.ssh_username || '',
    ssh_private_key: initialData?.connection.ssh_private_key || '',
    ssh_passphrase: initialData?.connection.ssh_passphrase || '',
    is_example_db: false
  });
  const [errors, setErrors] = useState<FormErrors>({});
  const [touched, setTouched] = useState<Record<string, boolean>>({});
  const [error, setError] = useState<string | null>(null);
  const [schemaValidationError, setSchemaValidationError] = useState<string | null>(null);
  const [autoExecuteQuery, setAutoExecuteQuery] = useState<boolean>(
    initialData?.settings.auto_execute_query !== undefined 
      ? initialData.settings.auto_execute_query 
      : true
  );
  const [shareWithAI, setShareWithAI] = useState<boolean>(
    initialData?.settings.share_data_with_ai !== undefined 
      ? initialData.settings.share_data_with_ai 
      : false
  );
  const [nonTechMode, setNonTechMode] = useState<boolean>(
    initialData?.settings.non_tech_mode !== undefined 
      ? initialData.settings.non_tech_mode 
      : false
  );
  // Refs for MongoDB URI inputs
  const mongoUriInputRef = useRef<HTMLInputElement>(null);
  const mongoUriSshInputRef = useRef<HTMLInputElement>(null);
  const credentialsTextAreaRef = useRef<HTMLTextAreaElement>(null);

  // Add these refs to store previous tab states
  const [, setTabsVisited] = useState<Record<ModalTab, boolean>>({
    connection: true,
    schema: false,
    settings: false
  });
  
  // State for MongoDB URI fields
  const [mongoUriValue, setMongoUriValue] = useState<string>('');
  const [mongoUriSshValue, setMongoUriSshValue] = useState<string>('');
  
  // State for credentials text area
  const [credentialsValue, setCredentialsValue] = useState<string>('');

  // Update autoExecuteQuery when initialData changes
  useEffect(() => {
    if (initialData) {
      if (initialData.settings.auto_execute_query !== undefined) {
        setAutoExecuteQuery(initialData.settings.auto_execute_query);
      }
      if (initialData.settings.share_data_with_ai !== undefined) {
        setShareWithAI(initialData.settings.share_data_with_ai);
      }
      if (initialData.settings.non_tech_mode !== undefined) {
        setNonTechMode(initialData.settings.non_tech_mode);
      }
      // Set the connection type tab based on whether SSH is enabled
      if (initialData.connection.ssh_enabled) {
        handleConnectionTypeChange('ssh');
      } else {
        handleConnectionTypeChange('basic');
      }

      // Initialize the credentials textarea with the connection string format
      const formattedConnectionString = formatConnectionString(initialData.connection);
      setCredentialsValue(formattedConnectionString);
      if (credentialsTextAreaRef.current) {
        credentialsTextAreaRef.current.value = formattedConnectionString;
      }

      // For MongoDB connections, also format the MongoDB URI for both tabs
      if (initialData.connection.type === 'mongodb') {
        const formatMongoURI = (connection: Connection): string => {
          const auth = connection.username ? 
            `${connection.username}${connection.password ? `:${connection.password}` : ''}@` : '';
          const srv = connection.host.includes('.mongodb.net') ? '+srv' : '';
          const portPart = srv ? '' : `:${connection.port || '27017'}`;
          const dbPart = connection.database ? `/${connection.database}` : '';
          const authSource = connection.auth_database ? `?authSource=${connection.auth_database}` : '';
          
          return `mongodb${srv}://${auth}${connection.host}${portPart}${dbPart}${authSource}`;
        };

        const mongoUri = formatMongoURI(initialData.connection);
        
        // Set the value for both URI inputs (basic and SSH tabs)
        setMongoUriValue(mongoUri);
        setMongoUriSshValue(mongoUri);
        
        if (mongoUriInputRef.current) {
          mongoUriInputRef.current.value = mongoUri;
        }
        
        if (mongoUriSshInputRef.current) {
          mongoUriSshInputRef.current.value = mongoUri;
        }
      }
    }
  }, [initialData]);

  // Load tables for Schema tab when editing an existing connection or after creating a new one
  useEffect(() => {
    // Load tables when schema tab is active and we have either initialData or a new connection
    const shouldLoadTables = 
      ((activeTab === 'schema' || (activeTab === 'connection' && formData.type === 'spreadsheet')) && 
      ((initialData && !tables.length) || (showingNewlyCreatedSchema && newChatId && !tables.length)));
    
    if (shouldLoadTables) {
      loadTables();
    }
  }, [initialData, activeTab, tables.length, showingNewlyCreatedSchema, newChatId, formData.type]);

  // Use useEffect to update the value of the MongoDB URI inputs when the tab changes
  useEffect(() => {
    if (formData.type === 'mongodb') {
      // Set the MongoDB URI input values
      if (mongoUriInputRef.current && mongoUriValue) {
        mongoUriInputRef.current.value = mongoUriValue;
      }
      
      if (mongoUriSshInputRef.current && mongoUriSshValue) {
        mongoUriSshInputRef.current.value = mongoUriSshValue;
      }
    }
    
    // Set the credentials textarea value
    if (credentialsTextAreaRef.current && credentialsValue) {
      credentialsTextAreaRef.current.value = credentialsValue;
    }
  }, [activeTab, formData.type, mongoUriValue, mongoUriSshValue, credentialsValue]);

  // Use useEffect to handle MongoDB URI persistence when switching connection types
  useEffect(() => {
    if (formData.type === 'mongodb') {
      // When switching from basic to SSH, ensure SSH MongoDB URI field gets the basic value
      if (prevConnectionType === 'basic' && connectionType === 'ssh' && mongoUriValue) {
        setMongoUriSshValue(mongoUriValue);
        if (mongoUriSshInputRef.current) {
          mongoUriSshInputRef.current.value = mongoUriValue;
        }
      }
      
      // When switching from SSH to basic, ensure basic MongoDB URI field gets the SSH value
      if (prevConnectionType === 'ssh' && connectionType === 'basic' && mongoUriSshValue) {
        setMongoUriValue(mongoUriSshValue);
        if (mongoUriInputRef.current) {
          mongoUriInputRef.current.value = mongoUriSshValue;
        }
      }
    }
  }, [connectionType, prevConnectionType, formData.type, mongoUriValue, mongoUriSshValue]);

  // Function to load tables for the Schema tab
  const loadTables = async () => {
    // Use newChatId when initialData is not available
    const chatId = initialData ? initialData.id : (showingNewlyCreatedSchema ? newChatId : undefined);
    if (!chatId) return;
    
    try {
      setIsLoadingTables(true);
      setError(null);
      setSchemaValidationError(null);
      
      const tablesResponse = await chatService.getTables(chatId);
      setTables(tablesResponse.tables || []);
      
      // Initialize selected tables based on is_selected field
      const selectedTableNames = tablesResponse.tables?.filter((table: TableInfo) => table.is_selected)
        .map((table: TableInfo) => table.name) || [];
      
      setSelectedTables(selectedTableNames);
      
      // Check if all tables are selected to set selectAll state correctly
      setSelectAllTables(selectedTableNames?.length === tablesResponse.tables?.length);
    } catch (error: any) {
      console.error('Failed to load tables:', error);
      setError(error.message || 'Failed to load tables');
    } finally {
      setIsLoadingTables(false);
    }
  };

  const validateField = (name: string, value: Connection) => {
    switch (name) {
      case 'host':
        if (!value.host.trim()) {
          return 'Host is required';
        }
        if (!/^[a-zA-Z0-9.-]+$/.test(value.host)) {
          return 'Invalid host format';
        }
        break;
      case 'port':
        // For MongoDB, port is optional and can be empty
        if (value.type === 'mongodb') {
          return '';
        }
        if (!value.port) {
          return 'Port is required';
        }
        
        const port = parseInt(value.port);
        if (isNaN(port) || port < 1 || port > 65535) {
          return 'Port must be between 1 and 65535';
        }
        break;
      case 'database':
        if (!value.database.trim()) {
          return 'Database name is required';
        }
        if (!/^[a-zA-Z0-9_-]+$/.test(value.database)) {
          return 'Invalid database name format';
        }
        break;
      case 'username':
        if (!value.username.trim()) {
          return 'Username is required';
        }
        break;
      case 'ssl_cert_url':
        if (value.use_ssl && value.ssl_mode !== 'disable' && value.ssl_mode !== 'require' && !value.ssl_cert_url?.trim()) {
          return 'SSL Certificate URL is required for this SSL mode';
        }
        if (value.ssl_cert_url && !isValidUrl(value.ssl_cert_url)) {
          return 'Invalid URL format';
        }
        break;
      case 'ssl_key_url':
        if (value.use_ssl && value.ssl_mode !== 'disable' && value.ssl_mode !== 'require' && !value.ssl_key_url?.trim()) {
          return 'SSL Key URL is required for this SSL mode';
        }
        if (value.ssl_key_url && !isValidUrl(value.ssl_key_url)) {
          return 'Invalid URL format';
        }
        break;
      case 'ssl_root_cert_url':
        if (value.use_ssl && value.ssl_mode !== 'disable' && value.ssl_mode !== 'require' && !value.ssl_root_cert_url?.trim()) {
          return 'SSL Root Certificate URL is required for this SSL mode';
        }
        if (value.ssl_root_cert_url && !isValidUrl(value.ssl_root_cert_url)) {
          return 'Invalid URL format';
        }
        break;
      // SSH validation
      case 'ssh_host':
        if (value.ssh_enabled && !value.ssh_host?.trim()) {
          return 'SSH Host is required';
        }
        if (value.ssh_host && !/^[a-zA-Z0-9.-]+$/.test(value.ssh_host)) {
          return 'Invalid SSH host format';
        }
        break;
      case 'ssh_port':
        if (value.ssh_enabled && !value.ssh_port) {
          return 'SSH Port is required';
        }
        if (value.ssh_port) {
          const sshPort = parseInt(value.ssh_port);
          if (isNaN(sshPort) || sshPort < 1 || sshPort > 65535) {
            return 'SSH Port must be between 1 and 65535';
          }
        }
        break;
      case 'ssh_username':
        if (value.ssh_enabled && !value.ssh_username?.trim()) {
          return 'SSH Username is required';
        }
        break;
      case 'ssh_private_key':
        if (value.ssh_enabled && !value.ssh_private_key?.trim()) {
          return 'SSH Private Key is required';
        }
        break;
      default:
        return '';
    }
  };

  // Helper function to validate URLs
  const isValidUrl = (url: string): boolean => {
    try {
      new URL(url);
      return true;
    } catch (e) {
      return false;
    }
  };

  // Update the handleUpdateSettings function to safely check auto_execute_query
  const handleUpdateSettings = async () => {
    if (!initialData || !onEdit) return;
    
    try {
      setIsLoading(true);
      setError(null);
      setSuccessMessage(null);
      // Update the settings via the API
      const result = await onEdit(undefined, {
        auto_execute_query: autoExecuteQuery,
        share_data_with_ai: shareWithAI,
        non_tech_mode: nonTechMode
      });
      
      if (result?.success) {
        // If we have updated chat data, sync our local state with it
        if (result.updatedChat) {
          setAutoExecuteQuery(result.updatedChat.settings.auto_execute_query);
          setShareWithAI(result.updatedChat.settings.share_data_with_ai);
          setNonTechMode(result.updatedChat.settings.non_tech_mode);
          setCurrentChatData(result.updatedChat);
        }
        
        // Show success message - will auto-dismiss after 3 seconds
        setSuccessMessage("Settings updated successfully");
      } else if (result?.error) {
        setError(result.error);
      }
    } catch (error: any) {
      console.error('Failed to update settings:', error);
      setError(error.message || 'Failed to update settings');
    } finally {
      setIsLoading(false);
    }
  };

  // Success message auto-dismiss timer
  useEffect(() => {
    let timer: NodeJS.Timeout;
    if (successMessage) {
      timer = setTimeout(() => {
        setSuccessMessage(null);
      }, 3000); // Clear success message after 3 seconds
    }
    return () => {
      if (timer) clearTimeout(timer);
    };
  }, [successMessage]);

  // Update handleSubmit to not close the modal automatically when updating connection
  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setIsLoading(true);
    setError(null);
    setSuccessMessage(null); // Clear any existing success messages

    // Handle schema updates when in schema tab
    if (activeTab === 'schema' && initialData) {
      await handleUpdateSchema();
      return;
    }

    // Handle settings updates when in settings tab
    if (activeTab === 'settings' && initialData) {
      await handleUpdateSettings();
      return;
    }

    // Update ssh_enabled based on current tab for connection updates
    const updatedFormData = {
      ...formData,
      ssh_enabled: connectionType === 'ssh'
    };
    setFormData(updatedFormData);

    // For connection tab, validate all fields first
    const newErrors: FormErrors = {};
    let hasErrors = false;

    // Skip validation for Spreadsheet types as they don't need connection details
    if (updatedFormData.type === 'spreadsheet') {
      // Validate that at least one file is uploaded
      if (!updatedFormData.file_uploads || updatedFormData.file_uploads.length === 0) {
        setError('Please upload at least one file');
        setIsLoading(false);
        return;
      }
    } else {
      // Always validate these fields for database connections
      ['host', 'port', 'database', 'username'].forEach(field => {
        const error = validateField(field, updatedFormData);
        if (error) {
          newErrors[field as keyof FormErrors] = error;
          hasErrors = true;
        }
      });
    }

    // Validate SSL fields if SSL is enabled in Basic mode
    if (connectionType === 'basic' && updatedFormData.use_ssl) {
      // For verify-ca and verify-full modes, we need certificates
      if (['verify-ca', 'verify-full'].includes(updatedFormData.ssl_mode || '')) {
        ['ssl_cert_url', 'ssl_key_url', 'ssl_root_cert_url'].forEach(field => {
          const error = validateField(field, updatedFormData);
          if (error) {
            newErrors[field as keyof FormErrors] = error;
            hasErrors = true;
          }
        });
      }
    }

    // Validate SSH fields if SSH tab is active
    if (connectionType === 'ssh') {
      ['ssh_host', 'ssh_port', 'ssh_username', 'ssh_private_key'].forEach(field => {
        const error = validateField(field, updatedFormData);
        if (error) {
          newErrors[field as keyof FormErrors] = error;
          hasErrors = true;
        }
      });
    }

    setErrors(newErrors);
    setTouched({
      host: true,
      port: true,
      database: true,
      username: true,
      ...(updatedFormData.use_ssl && connectionType === 'basic' ? {
        ssl_cert_url: true,
        ssl_key_url: true,
        ssl_root_cert_url: true
      } : {}),
      ...(connectionType === 'ssh' ? {
        ssh_host: true,
        ssh_port: true,
        ssh_username: true,
        ssh_private_key: true
      } : {})
    });

    if (hasErrors) {
      setIsLoading(false);
      return;
    }

    try {
      // Handle file uploads for newly created spreadsheet connections
      if (showingNewlyCreatedSchema && newChatId && formData.type === 'spreadsheet' && fileUploads.length > 0) {
        try {
          setSuccessMessage("Uploading files...");
          
          // Upload each file
          for (const fileUpload of fileUploads) {
            if (!fileUpload.file) {
              console.error('No file object found for upload:', fileUpload.filename);
              continue;
            }
            
            const formData = new FormData();
            formData.append('file', fileUpload.file);
            formData.append('tableName', fileUpload.tableName || '');
            formData.append('mergeStrategy', fileUpload.mergeStrategy || 'replace');
            
            // Add merge options if present
            if (fileUpload.mergeOptions) {
              formData.append('ignoreCase', String(fileUpload.mergeOptions.ignoreCase ?? true));
              formData.append('trimWhitespace', String(fileUpload.mergeOptions.trimWhitespace ?? true));
              formData.append('handleNulls', fileUpload.mergeOptions.handleNulls || 'empty');
              formData.append('addNewColumns', String(fileUpload.mergeOptions.addNewColumns ?? true));
              formData.append('dropMissingColumns', String(fileUpload.mergeOptions.dropMissingColumns ?? false));
              formData.append('updateExisting', String(fileUpload.mergeOptions.updateExisting ?? true));
              formData.append('insertNew', String(fileUpload.mergeOptions.insertNew ?? true));
              formData.append('deleteMissing', String(fileUpload.mergeOptions.deleteMissing ?? false));
            }
            
            const response = await fetch(`${import.meta.env.VITE_API_URL}/upload/${newChatId}/file`, {
              method: 'POST',
              headers: {
                'Authorization': `Bearer ${localStorage.getItem('token')}`
              },
              body: formData
            });
            
            if (!response.ok) {
              const errorData = await response.json();
              console.error('Upload error response:', errorData);
              throw new Error(errorData.error || `Failed to upload file: ${response.status} ${response.statusText}`);
            }
          }
          
          setSuccessMessage("Files uploaded successfully. Loading data structure...");
          
          // Wait a bit for the backend to process the files
          await new Promise(resolve => setTimeout(resolve, 1000));
          
          // Switch to schema tab to show the new tables
          setActiveTab('schema');
          
          // Clear the uploaded files from the state
          setFileUploads([]);
          setFormData(prev => ({ ...prev, file_uploads: [] }));
          
          // Force reload tables after upload
          setTables([]); // Clear existing tables to force refresh
          await loadTables();
          
          // Refresh the chat data to get updated database name
          try {
            const updatedChat = await chatService.getChat(newChatId);
            setCurrentChatData(updatedChat);
            // Update the form data with the new database name
            setFormData(prev => ({
              ...prev,
              database: updatedChat.connection.database
            }));
          } catch (error) {
            console.error('Failed to refresh chat data:', error);
          }
          
          // Show final success message
          setSuccessMessage("Files uploaded and tables loaded successfully");
          setIsLoading(false);
          return;
        } catch (error: any) {
          console.error('Failed to upload files:', error);
          setError(error.message || 'Failed to upload files');
          setIsLoading(false);
          return;
        }
      }
      
      if (initialData) {
        // Check if critical connection details have changed
        const credentialsChanged = 
          initialData.connection.database !== updatedFormData.database ||
          initialData.connection.host !== updatedFormData.host ||
          initialData.connection.port !== updatedFormData.port ||
          initialData.connection.username !== updatedFormData.username;

        const result = await onEdit?.(updatedFormData, { 
          auto_execute_query: autoExecuteQuery, 
          share_data_with_ai: shareWithAI,
          non_tech_mode: nonTechMode 
        });
        console.log("edit result in connection modal", result);
        if (result?.success) {
          // If we have updated chat data, sync our local state with it
          if (result.updatedChat) {
            setAutoExecuteQuery(result.updatedChat.settings.auto_execute_query);
            setShareWithAI(result.updatedChat.settings.share_data_with_ai);
            setNonTechMode(result.updatedChat.settings.non_tech_mode);
            setCurrentChatData(result.updatedChat);
            // Update the form data with the new database name for spreadsheets
            if (result.updatedChat?.connection.type === 'spreadsheet') {
              setFormData(prev => ({
                ...prev,
                database: result.updatedChat!.connection.database
              }));
            }
          }
          
          // For spreadsheet connections with new files, upload them
          if (updatedFormData.type === 'spreadsheet' && fileUploads.length > 0) {
            try {
              setSuccessMessage("Uploading files...");
              
              // Upload each file
              for (const fileUpload of fileUploads) {
                if (!fileUpload.file) {
                  console.error('No file object found for upload:', fileUpload.filename);
                  continue;
                }
                
                const formData = new FormData();
                formData.append('file', fileUpload.file);
                formData.append('tableName', fileUpload.tableName || '');
                formData.append('mergeStrategy', fileUpload.mergeStrategy || 'replace');
                
                // Add merge options if present
                if (fileUpload.mergeOptions) {
                  formData.append('ignoreCase', String(fileUpload.mergeOptions.ignoreCase ?? true));
                  formData.append('trimWhitespace', String(fileUpload.mergeOptions.trimWhitespace ?? true));
                  formData.append('handleNulls', fileUpload.mergeOptions.handleNulls || 'empty');
                  formData.append('addNewColumns', String(fileUpload.mergeOptions.addNewColumns ?? true));
                  formData.append('dropMissingColumns', String(fileUpload.mergeOptions.dropMissingColumns ?? false));
                  formData.append('updateExisting', String(fileUpload.mergeOptions.updateExisting ?? true));
                  formData.append('insertNew', String(fileUpload.mergeOptions.insertNew ?? true));
                  formData.append('deleteMissing', String(fileUpload.mergeOptions.deleteMissing ?? false));
                }
                
                const response = await fetch(`${import.meta.env.VITE_API_URL}/upload/${initialData.id}/file`, {
                  method: 'POST',
                  headers: {
                    'Authorization': `Bearer ${localStorage.getItem('token')}`
                  },
                  body: formData
                });
                
                if (!response.ok) {
                  const errorData = await response.json();
                  console.error('Upload error response:', errorData);
                  throw new Error(errorData.error || `Failed to upload file: ${response.status} ${response.statusText}`);
                }
              }
              
              setSuccessMessage("Files uploaded successfully. Loading data structure...");
              
              // Clear the uploaded files from the state
              setFileUploads([]);
              setFormData(prev => ({ ...prev, file_uploads: [] }));
              
              // Wait a bit for the backend to process the files
              await new Promise(resolve => setTimeout(resolve, 1000));
              
              // Switch to schema tab to show the new tables
              setActiveTab('schema');
              
              // Force reload tables after upload
              setTables([]); // Clear existing tables to force refresh
              await loadTables();
              
              // Refresh the chat data to get updated database name
              try {
                const updatedChat = await chatService.getChat(initialData.id);
                setCurrentChatData(updatedChat);
                // Update the form data with the new database name
                setFormData(prev => ({
                  ...prev,
                  database: updatedChat.connection.database
                }));
              } catch (error) {
                console.error('Failed to refresh chat data:', error);
              }
              
              // Show final success message
              setSuccessMessage("Files uploaded and tables loaded successfully");
              setIsLoading(false);
            } catch (error: any) {
              console.error('Failed to upload files:', error);
              setError(error.message || 'Failed to upload files');
              setIsLoading(false);
              return;
            }
          } else if (credentialsChanged && activeTab === 'connection') {
            // If credentials changed and we're in the connection tab, switch to schema tab
            setActiveTab('schema');
            // Load tables
            loadTables();
            setIsLoading(false);
          } else {
            // Show success message - will auto-dismiss after 3 seconds
            setSuccessMessage("Connection updated successfully");
            setIsLoading(false);
          }
        } else if (result?.error) {
          setError(result.error);
          setIsLoading(false);
        }
      } else {
        // For new connections, pass settings to onSubmit
        const result = await onSubmit(updatedFormData, { 
          auto_execute_query: autoExecuteQuery, 
          share_data_with_ai: shareWithAI,
          non_tech_mode: nonTechMode 
        });
        console.log("submit result in connection modal", result);
        if (result?.success) {
          if (result.chatId) {
            // Store the new chat ID for use in handleUpdateSchema
            setNewChatId(result.chatId);
            setShowingNewlyCreatedSchema(true);
            
            // For spreadsheet connections, establish connection first then upload files
            if (updatedFormData.type === 'spreadsheet' && fileUploads.length > 0) {
              try {
                setSuccessMessage("Establishing connection...");
                
                // First, establish the database connection
                const connectResponse = await fetch(`${import.meta.env.VITE_API_URL}/chats/${result.chatId}/connect`, {
                  method: 'POST',
                  headers: {
                    'Content-Type': 'application/json',
                    'Authorization': `Bearer ${localStorage.getItem('token')}`
                  },
                  body: JSON.stringify({
                    stream_id: generateStreamId()
                  })
                });
                
                if (!connectResponse.ok) {
                  const error = await connectResponse.json();
                  throw new Error(error.error || 'Failed to establish database connection');
                }
                
                // Wait a bit for connection to stabilize
                await new Promise(resolve => setTimeout(resolve, 500));
                
                setSuccessMessage("Uploading files...");
                
                // Upload each file
                for (const fileUpload of fileUploads) {
                  if (!fileUpload.file) {
                    console.error('No file object found for upload:', fileUpload.filename);
                    continue;
                  }
                  
                  const formData = new FormData();
                  formData.append('file', fileUpload.file);
                  formData.append('tableName', fileUpload.tableName || '');
                  formData.append('mergeStrategy', fileUpload.mergeStrategy || 'replace');
                  
                  // Add merge options if present
                  if (fileUpload.mergeOptions) {
                    formData.append('ignoreCase', String(fileUpload.mergeOptions.ignoreCase ?? true));
                    formData.append('trimWhitespace', String(fileUpload.mergeOptions.trimWhitespace ?? true));
                    formData.append('handleNulls', fileUpload.mergeOptions.handleNulls || 'empty');
                    formData.append('addNewColumns', String(fileUpload.mergeOptions.addNewColumns ?? true));
                    formData.append('dropMissingColumns', String(fileUpload.mergeOptions.dropMissingColumns ?? false));
                    formData.append('updateExisting', String(fileUpload.mergeOptions.updateExisting ?? true));
                    formData.append('insertNew', String(fileUpload.mergeOptions.insertNew ?? true));
                    formData.append('deleteMissing', String(fileUpload.mergeOptions.deleteMissing ?? false));
                  }
                  
                  const response = await fetch(`${import.meta.env.VITE_API_URL}/upload/${result.chatId}/file`, {
                    method: 'POST',
                    headers: {
                      'Authorization': `Bearer ${localStorage.getItem('token')}`
                    },
                    body: formData
                  });
                  
                  if (!response.ok) {
                    const errorData = await response.json();
                    console.error('Upload error response:', errorData);
                    throw new Error(errorData.error || `Failed to upload file: ${response.status} ${response.statusText}`);
                  }
                }
                
                setSuccessMessage("Files uploaded successfully. Loading data structure...");
              } catch (error: any) {
                console.error('Failed to upload files:', error);
                setError(error.message || 'Failed to upload files');
                setIsLoading(false);
                return;
              }
            }
            
            // Switch to schema tab
            setActiveTab('schema');
            
            // Set isLoadingTables to true while fetching schema data
            setIsLoadingTables(true);
            
            // Load the tables for the new connection
            try {
              const tablesResponse = await chatService.getTables(result.chatId);
              setTables(tablesResponse.tables || []);
              
              // Initialize selected tables based on is_selected field
              const selectedTableNames = tablesResponse.tables?.filter((table: TableInfo) => table.is_selected)
                .map((table: TableInfo) => table.name) || [];
              
              setSelectedTables(selectedTableNames);
              
              // Check if all tables are selected to set selectAll state correctly
              setSelectAllTables(selectedTableNames?.length === tablesResponse.tables?.length);
              
              // For spreadsheet connections, refresh the chat data to get updated database name
              if (updatedFormData.type === 'spreadsheet') {
                try {
                  const updatedChat = await chatService.getChat(result.chatId);
                  setCurrentChatData(updatedChat);
                  // Update the form data with the new database name
                  setFormData(prev => ({
                    ...prev,
                    database: updatedChat.connection.database
                  }));
                } catch (error) {
                  console.error('Failed to refresh chat data:', error);
                }
              }
              
              console.log('Connection created. Now you can select tables to include in your schema.');
              setSuccessMessage(updatedFormData.type === 'spreadsheet' 
                ? "Files uploaded successfully. Review your data structure."
                : "Connection created successfully. Select tables to include in your schema.");
            } catch (error: any) {
              console.error('Failed to load tables for new connection:', error);
              setError(error.message || 'Failed to load tables for new connection');
            } finally {
              setIsLoadingTables(false);
              setIsLoading(false);
            }
          } else {
            onClose();
          }
        } else if (result?.error) {
          setError(result.error);
          setIsLoading(false);
        }
      }
    } catch (err: any) {
      setError(err.message || 'An error occurred while updating the connection');
      setIsLoading(false);
    }
  };


  const handleChange = (
    e: React.ChangeEvent<HTMLInputElement | HTMLSelectElement>
  ) => {
    const { name, value } = e.target;
    
    // Special handling for type change to spreadsheet
    if (name === 'type' && value === 'spreadsheet') {
      setFormData((prev) => ({
        ...prev,
        [name]: value,
        database: currentChatData?.connection.database || 'spreadsheet_db',
        host: 'internal-spreadsheet',
        port: '0',
        username: 'spreadsheet_user',
        password: 'internal'
      }));
    } else {
      setFormData((prev) => ({
        ...prev,
        [name]: value,
      }));
    }

    if (touched[name]) {
      const error = validateField(name, {
        ...formData,
        [name]: value,
      });
      setErrors(prev => ({
        ...prev,
        [name]: error,
      }));
    }
  };

  const handleBlur = (e: React.FocusEvent<HTMLInputElement>) => {
    const { name } = e.target;
    setTouched(prev => ({
      ...prev,
      [name]: true,
    }));
    const error = validateField(name, formData);
    setErrors(prev => ({
      ...prev,
      [name]: error,
    }));
  };

  // Removed unused parseConnectionString function

  const formatConnectionString = (connection: Connection): string => {
    let result = `DATABASE_TYPE=${connection.type}
DATABASE_HOST=${connection.host}
DATABASE_PORT=${connection.port}
DATABASE_NAME=${connection.database}
DATABASE_USERNAME=${connection.username}
DATABASE_PASSWORD=`; // Mask password

    // Add SSL configuration if enabled
    if (connection.use_ssl) {
      result += `\nUSE_SSL=true`;
      result += `\nSSL_MODE=${connection.ssl_mode || 'disable'}`;
      
      if (connection.ssl_cert_url) {
        result += `\nSSL_CERT_URL=${connection.ssl_cert_url}`;
      }
      
      if (connection.ssl_key_url) {
        result += `\nSSL_KEY_URL=${connection.ssl_key_url}`;
      }
      
      if (connection.ssl_root_cert_url) {
        result += `\nSSL_ROOT_CERT_URL=${connection.ssl_root_cert_url}`;
      }
    }
    
    // Add SSH configuration if enabled
    if (connection.ssh_enabled) {
      result += `\nSSH_ENABLED=true`;
      result += `\nSSH_HOST=${connection.ssh_host || ''}`;
      result += `\nSSH_PORT=${connection.ssh_port || '22'}`;
      result += `\nSSH_USERNAME=${connection.ssh_username || ''}`;
      result += `\nSSH_PRIVATE_KEY=`; // Mask private key
      
      if (connection.ssh_passphrase) {
        result += `\nSSH_PASSPHRASE=`; // Mask passphrase
      }
    }
    
    return result;
  };

  // Schema tab functions
  const toggleTable = (tableName: string) => {
    setSchemaValidationError(null);
    setSelectedTables(prev => {
      if (prev.includes(tableName)) {
        // If removing a table, also uncheck "Select All"
        setSelectAllTables(false);
        
        // Prevent removing if it's the last selected table
        if (prev.length === 1) {
          setSchemaValidationError("At least one table must be selected");
          return prev;
        }
        
        return prev.filter(name => name !== tableName);
      } else {
        // If all tables are now selected, check "Select All"
        const newSelected = [...prev, tableName];
        if (newSelected.length === tables?.length) {
          setSelectAllTables(true);
        }
        return newSelected;
      }
    });
  };

  const toggleExpandTable = (tableName: string, forceState?: boolean) => {
    if (tableName === '') {
      // This is a special case for toggling all tables
      const allExpanded = Object.values(expandedTables).every(v => v);
      const newExpandedState = forceState !== undefined ? forceState : !allExpanded;
      
      const newExpandedTables = tables.reduce((acc, table) => {
        acc[table.name] = newExpandedState;
        return acc;
      }, {} as Record<string, boolean>);
      
      setExpandedTables(newExpandedTables);
    } else {
      // Toggle a single table
      setExpandedTables(prev => ({
        ...prev,
        [tableName]: forceState !== undefined ? forceState : !prev[tableName]
      }));
    }
  };

  const toggleSelectAllTables = () => {
    setSchemaValidationError(null);
    if (selectAllTables) {
      // Prevent deselecting all tables
      setSchemaValidationError("At least one table must be selected");
      return;
    } else {
      // Select all
      setSelectedTables(tables?.map(table => table.name) || []);
      setSelectAllTables(true);
    }
  };

  // Update handleUpdateSchema to close the modal when schema is submitted for a new connection
  const handleUpdateSchema = async () => {
    if (!initialData && !showingNewlyCreatedSchema) return;
    
    // For spreadsheet connections, just close the modal since data is already saved
    if (formData.type === 'spreadsheet') {
      setSuccessMessage("Data structure saved successfully");
      // Give the success message time to show before closing
      setTimeout(() => {
        onClose(currentChatData);
      }, 500);
      return;
    }
    
    // Validate that at least one table is selected
    if (selectedTables?.length === 0) {
      setSchemaValidationError("At least one table must be selected");
      return;
    }
    
    try {
      setIsLoading(true);
      setError(null);
      setSchemaValidationError(null);
      setSuccessMessage(null);
      
      // Format selected tables as "ALL" or comma-separated list
      const formattedSelection = selectAllTables ? 'ALL' : selectedTables.join(',');
      
      // Determine which chatId to use
      const chatId = showingNewlyCreatedSchema ? newChatId : initialData!.id;
      
      // Always save the selection, regardless of whether it has changed
      if (onUpdateSelectedCollections && chatId) {
        await onUpdateSelectedCollections(chatId, formattedSelection);
        
        // Show success message - will auto-dismiss after 3 seconds
        setSuccessMessage("Schema selection updated successfully");
        
        // If this is a new connection (no initialData), close the modal after updating schema
        if (!initialData && showingNewlyCreatedSchema) {
          // Give the success message time to show before closing
          setTimeout(() => {
            onClose(currentChatData);
          }, 1000);
        }
        
        // Log success
        console.log('Schema selection updated successfully');
      }
    } catch (error: any) {
      console.error('Failed to update selected tables:', error);
      setError(error.message || 'Failed to update selected tables');
    } finally {
      setIsLoading(false);
    }
  };

  // Handle tab changes
  const handleTabChange = (tab: ModalTab) => {
    setTabsVisited(prev => ({
      ...prev,
      [tab]: true
    }));
    setActiveTab(tab);
  };

  // Custom function to handle connection type change
  const handleConnectionTypeChange = (type: ConnectionType) => {
    setPrevConnectionType(connectionType);
    setConnectionType(type);
  };

  const renderTabContent = () => {
    switch (activeTab) {
      case 'connection':
        return (
          <>
            {/* Data Source Type Selector - Moved from BasicConnectionTab */}
            <div className="mb-6">
              <label className="block font-bold mb-2 text-lg">Data Source Type</label>
              <p className="text-gray-600 text-sm mb-2">Select your data source</p>
              <div className="relative">
                <select
                  name="type"
                  value={formData.type}
                  onChange={handleChange}
                  className="neo-input w-full appearance-none pr-12"
                >
                  {[
                    { value: 'postgresql', label: 'PostgreSQL' },
                    { value: 'yugabytedb', label: 'YugabyteDB' },
                    { value: 'mysql', label: 'MySQL' },
                    { value: 'clickhouse', label: 'ClickHouse' },
                    { value: 'mongodb', label: 'MongoDB' },
                    { value: 'spreadsheet', label: 'Spreadsheet Files (CSV, Excel)' },
                  ].map(option => (
                    <option key={option.value} value={option.value}>
                      {option.label}
                    </option>
                  ))}
                </select>
                <ChevronDown className="absolute right-3 top-1/2 transform -translate-y-1/2 w-5 h-5 text-gray-500 pointer-events-none" />
              </div>
            </div>
            
            {/* {formData.type !== 'spreadsheet' && (
              <div className="my-6 border-t border-gray-200"></div>
            )} */}
            
            {/* Connection type tabs - Hide for CSV/Excel */}
            {formData.type !== 'spreadsheet' && (
              <div className="flex border-b border-gray-200 mb-6">
                <button
                  type="button"
                  className={`py-2 px-4 font-semibold border-b-2 ${
                    connectionType === 'basic'
                      ? 'border-black text-black'
                      : 'border-transparent text-gray-500 hover:text-gray-700'
                  }`}
                  onClick={() => handleConnectionTypeChange('basic')}
                >
                  <div className="flex items-center gap-2">
                    <Monitor className="w-4 h-4" />
                    <span>Basic Connection</span>
                  </div>
                </button>
                <button
                  type="button"
                  className={`py-2 px-4 font-semibold border-b-2 ${
                    connectionType === 'ssh'
                      ? 'border-black text-black'
                      : 'border-transparent text-gray-500 hover:text-gray-700'
                  }`}
                  onClick={() => handleConnectionTypeChange('ssh')}
                >
                  <div className="flex items-center gap-2">
                    <KeyRound className="w-4 h-4" />
                    <span>SSH Tunnel</span>
                  </div>
                </button>
              </div>
            )}

            {/* Connection Tabs Content */}
            {formData.type === 'spreadsheet' ? (
              <FileUploadTab
                formData={formData}
                handleChange={handleChange}
                onFilesChange={(files) => {
                  setFileUploads(files);
                  setFormData(prev => ({ ...prev, file_uploads: files }));
                }}
                isEditMode={!!initialData || (showingNewlyCreatedSchema && !!newChatId)}
                chatId={initialData?.id || newChatId}
                preloadedTables={tables}
              />
            ) : connectionType === 'basic' ? (
              <BasicConnectionTab
                formData={formData}
                errors={errors}
                touched={touched}
                handleChange={handleChange}
                handleBlur={handleBlur}
                validateField={(name, value) => validateField(name, value)}
                mongoUriInputRef={mongoUriInputRef}
                onMongoUriChange={(uri) => setMongoUriValue(uri)}
              />
            ) : (
              <SSHConnectionTab
                formData={formData}
                errors={errors}
                touched={touched}
                handleChange={handleChange}
                handleBlur={handleBlur}
                validateField={(name, value) => validateField(name, value)}
                mongoUriSshInputRef={mongoUriSshInputRef}
                onMongoUriChange={(uri) => setMongoUriSshValue(uri)}
              />
            )}
          </>
        );
      case 'schema':
        return formData.type === 'spreadsheet' ? (
          <DataStructureTab
            chatId={newChatId || initialData?.id || ''}
            isLoadingData={isLoadingTables}
            onDeleteTable={(tableName) => {
              // TODO: Implement delete table functionality
              console.log('Delete table:', tableName);
            }}
            onDownloadData={(tableName) => {
              // TODO: Implement download data functionality
              console.log('Download data:', tableName);
            }}
            onRefreshData={() => {
              // Refresh the tables
              loadTables();
            }}
          />
        ) : (
          <SchemaTab
            isLoadingTables={isLoadingTables}
            tables={tables}
            selectedTables={selectedTables}
            expandedTables={expandedTables}
            schemaSearchQuery={schemaSearchQuery}
            selectAllTables={selectAllTables}
            schemaValidationError={schemaValidationError}
            setSchemaSearchQuery={setSchemaSearchQuery}
            toggleSelectAllTables={toggleSelectAllTables}
            toggleExpandTable={toggleExpandTable}
            toggleTable={toggleTable}
          />
        );
      case 'settings':
        return (
          <SettingsTab
            autoExecuteQuery={autoExecuteQuery}
            shareWithAI={shareWithAI}
            nonTechMode={nonTechMode}
            setAutoExecuteQuery={setAutoExecuteQuery}
            setShareWithAI={setShareWithAI}
            setNonTechMode={setNonTechMode}
          />
        );
      default:
        return null;
    }
  };

  return (
    <div className="fixed inset-0 bg-black/50 backdrop-blur-sm flex items-center justify-center p-4 z-[200]">
        <div className="bg-white neo-border rounded-lg w-full max-w-[40rem] max-h-[90vh] flex flex-col relative z-[201]">
          <div className="flex justify-between items-center p-6 border-b-4 border-black mb-2.5 flex-shrink-0">
            <div className="flex items-center gap-3">
              <Database className="w-6 h-6" />
              <div className="flex flex-col gap-1 mt-2">
                <h2 className="text-2xl font-bold">
                  {initialData ? 'Edit Connection' : 'New Connection'}
                  {(currentChatData || (showingNewlyCreatedSchema && formData.type === 'spreadsheet')) && formData.database && formData.database !== 'spreadsheet_db' && (
                    <span className="text-lg font-normal text-gray-600 ml-2">- {formData.database}</span>
                  )}
                </h2>
                <p className="text-gray-500 text-sm">Your data source credentials are stored in <strong>encrypted form</strong>.</p>
              </div>
            </div>
            <button
              onClick={() => onClose(currentChatData)}
              className="hover:bg-neo-gray rounded-lg p-2 transition-colors"
            >
              <X className="w-6 h-6" />
            </button>
          </div>
        
        {/* Main Tabs Navigation */}
        <div className="flex border-b border-gray-200 px-2 flex-shrink-0">
          <button
            type="button"
            className={`py-2 px-4 font-semibold border-b-2 ${
              activeTab === 'connection'
                ? 'border-black text-black'
                : 'border-transparent text-gray-500 hover:text-gray-700'
            }`}
            onClick={() => handleTabChange('connection')}
          >
            <div className="flex items-center gap-2">
              <Database className="w-4 h-4" />
              <span className="hidden md:block">Connection</span>
            </div>
          </button>
          
          {(initialData || showingNewlyCreatedSchema) && (
            <button
              type="button"
              className={`py-2 px-4 font-semibold border-b-2 ${
                activeTab === 'schema'
                  ? 'border-black text-black'
                  : 'border-transparent text-gray-500 hover:text-gray-700'
              }`}
              onClick={() => handleTabChange('schema')}
            >
              <div className="flex items-center gap-2">
                <Table className="w-4 h-4" />
                <span className="hidden md:block">{formData.type === 'spreadsheet' ? 'Data Structure' : 'Schema'}</span>
              </div>
            </button>
          )}
          
          <button
            type="button"
            className={`py-2 px-4 font-semibold border-b-2 ${
              activeTab === 'settings'
                ? 'border-black text-black'
                : 'border-transparent text-gray-500 hover:text-gray-700'
            }`}
            onClick={() => handleTabChange('settings')}
          >
            <div className="flex items-center gap-2">
              <Settings className="w-4 h-4" />
              <span className="hidden md:block">Settings</span>
            </div>
          </button>
        </div>

      <div className="overflow-y-auto thin-scrollbar flex-1 p-6">
        {renderTabContent()}
      </div>

      <form onSubmit={handleSubmit} className="p-6 pt-2 space-y-6 flex-shrink-0 border-t border-gray-200">
        {error && (
          <div className="p-4 mt-2 -mb-2 bg-red-50 border-2 border-red-500 rounded-lg">
            <div className="flex items-center gap-2 text-red-600">
              <AlertCircle className="w-5 h-5" />
              <p className="font-medium">{error}</p>
            </div>
          </div>
        )}

        {/* Form Submit and Cancel Buttons - Show in all tabs except when creating a new connection or when loading tables */}
        {(activeTab === 'connection' || activeTab === 'settings' || (activeTab === 'schema'  && !isLoadingTables)) && (
          <>
            {/* Password notice for updating connections */}
            {initialData && !successMessage && !isLoading && activeTab === 'connection' && formData.type !== 'spreadsheet' && (
              <div className="mt-2 -mb-2 p-3 bg-yellow-50 border-l-4 border-yellow-500 rounded">
                <div className="flex items-center gap-2">
                  <AlertCircle className="w-5 h-5 text-yellow-500 flex-shrink-0" />
                  <p className="text-sm font-medium">
                    <span className="text-yellow-700">Important:</span> To update your connection, you must re-enter your database password.
                  </p>
                </div>
              </div>
            )}
            
          {successMessage && (
            <div className="mt-2 -mb-2 p-3 bg-green-50 border-2 border-green-500 rounded-lg">
              <div className="flex items-center gap-2 text-green-600">
                <CheckCircle className="w-4 h-4" />
                <p className="text-sm font-medium">{successMessage}</p>
              </div>
            </div>
          )}
          
          <div className="flex flex-col md:flex-row gap-4 mt-3">
            <button
              type={activeTab === 'connection' ? 'submit' : 'button'}
              onClick={
                activeTab === 'schema' 
                  ? handleUpdateSchema 
                  : activeTab === 'settings'
                    ? handleUpdateSettings
                    : undefined
              }
              className="neo-button flex-1 relative"
              disabled={isLoading}
            >
              {isLoading ? (
                <div className="flex items-center justify-center gap-2">
                  <Loader2 className="w-4 h-4 animate-spin" />
                  <span>{initialData ? 'Updating...' : 'Creating...'}</span>
                </div>
              ) : (
                <span>
                  {!initialData 
                    ? (showingNewlyCreatedSchema && activeTab === 'schema') 
                      ? (formData.type === 'spreadsheet' ? 'Save Structure' : 'Save Schema') 
                      : (showingNewlyCreatedSchema && activeTab === 'connection' && formData.type === 'spreadsheet')
                        ? 'Upload Files'
                        : 'Create' 
                    : activeTab === 'settings' 
                      ? 'Update Settings' 
                      : activeTab === 'schema' 
                        ? (formData.type === 'spreadsheet' ? 'Update Structure' : 'Update Schema') 
                        : (showingNewlyCreatedSchema && formData.type === 'spreadsheet' && fileUploads.length > 0)
                          ? 'Upload Files'
                          : 'Update Connection'}
                </span>
              )}
            </button>
            <button
              type="button"
              onClick={() => onClose(currentChatData)}
              className="neo-button-secondary flex-1"
              disabled={isLoading}
            >
              Close
            </button>
          </div>
          </>
        )}
      </form>
    </div>
  </div>
);
}