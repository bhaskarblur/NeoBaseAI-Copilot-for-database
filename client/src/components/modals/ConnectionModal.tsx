import { AlertCircle, CheckCircle, Database, KeyRound, Loader2, Monitor, Settings, Table, X } from 'lucide-react';
import React, { useEffect, useRef, useState } from 'react';
import { Chat, ChatSettings, Connection, SSLMode, TableInfo } from '../../types/chat';
import chatService from '../../services/chatService';
import { BasicConnectionTab, SchemaTab, SettingsTab, SSHConnectionTab } from './components';

// Connection tab type
type ConnectionType = 'basic' | 'ssh';

// Modal tab type
type ModalTab = 'connection' | 'schema' | 'settings';

interface ConnectionModalProps {
  initialData?: Chat;
  onClose: () => void;
  onEdit?: (data?: Connection, settings?: ChatSettings) => Promise<{ success: boolean, error?: string }>;
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
  onClose, 
  onEdit, 
  onSubmit,
  onUpdateSelectedCollections,
}: ConnectionModalProps) {
  // Modal tab state to toggle between Connection, Schema, and Settings
  const [activeTab, setActiveTab] = useState<ModalTab>('connection');
  
  // Connection type state to toggle between basic and SSH tabs (within Connection tab)
  const [connectionType, setConnectionType] = useState<ConnectionType>('basic');
  
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
    database: initialData?.connection.database || '',
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
      activeTab === 'schema' && 
      ((initialData && !tables.length) || (showingNewlyCreatedSchema && newChatId && !tables.length));
    
    if (shouldLoadTables) {
      loadTables();
    }
  }, [initialData, activeTab, tables.length, showingNewlyCreatedSchema, newChatId]);

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
        share_data_with_ai: shareWithAI
      });
      
      if (result?.success) {
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

    // Always validate these fields
    ['host', 'port', 'database', 'username'].forEach(field => {
      const error = validateField(field, updatedFormData);
      if (error) {
        newErrors[field as keyof FormErrors] = error;
        hasErrors = true;
      }
    });

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
      if (initialData) {
        // Check if critical connection details have changed
        const credentialsChanged = 
          initialData.connection.database !== updatedFormData.database ||
          initialData.connection.host !== updatedFormData.host ||
          initialData.connection.port !== updatedFormData.port ||
          initialData.connection.username !== updatedFormData.username;

        const result = await onEdit?.(updatedFormData, { 
          auto_execute_query: autoExecuteQuery, 
          share_data_with_ai: shareWithAI 
        });
        console.log("edit result in connection modal", result);
        if (result?.success) {
          // If credentials changed and we're in the connection tab, switch to schema tab
          if (credentialsChanged && activeTab === 'connection') {
            setActiveTab('schema');
            // Load tables
            loadTables();
          } else {
            // Show success message - will auto-dismiss after 3 seconds
            setSuccessMessage("Connection updated successfully");
          }
        } else if (result?.error) {
          setError(result.error);
        }
      } else {
        // For new connections, pass settings to onSubmit
        const result = await onSubmit(updatedFormData, { 
          auto_execute_query: autoExecuteQuery, 
          share_data_with_ai: shareWithAI 
        });
        console.log("submit result in connection modal", result);
        if (result?.success) {
          if (result.chatId) {
            // Store the new chat ID for use in handleUpdateSchema
            setNewChatId(result.chatId);
            setShowingNewlyCreatedSchema(true);
            
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
              
              console.log('Connection created. Now you can select tables to include in your schema.');
              setSuccessMessage("Connection created successfully. Select tables to include in your schema.");
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
    setFormData((prev) => ({
      ...prev,
      [name]: value,
    }));

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

  const parseConnectionString = (text: string): Partial<Connection> => {
    const result: Partial<Connection> = {};
    const lines = text.split('\n');

    lines.forEach(line => {
      const [key, value] = line.split('=').map(s => s.trim());
      switch (key) {
        case 'DATABASE_TYPE':
          if (['postgresql', 'yugabytedb', 'mysql', 'clickhouse', 'mongodb', 'redis', 'neo4j'].includes(value)) {
            result.type = value as 'postgresql' | 'yugabytedb' | 'mysql' | 'clickhouse' | 'mongodb' | 'redis' | 'neo4j';
          }
          break;
        case 'DATABASE_HOST':
          result.host = value;
          break;
        case 'DATABASE_PORT':
          result.port = value;
          break;
        case 'DATABASE_NAME':
          result.database = value;
          break;
        case 'DATABASE_USERNAME':
          result.username = value;
          break;
        case 'DATABASE_PASSWORD':
          result.password = value;
          break;
        case 'USE_SSL':
          result.use_ssl = value.toLowerCase() === 'true';
          break;
        case 'SSL_MODE':
          if (['disable', 'require', 'verify-ca', 'verify-full'].includes(value)) {
            result.ssl_mode = value as SSLMode;
          }
          break;
        case 'SSL_CERT_URL':
          result.ssl_cert_url = value;
          break;
        case 'SSL_KEY_URL':
          result.ssl_key_url = value;
          break;
        case 'SSL_ROOT_CERT_URL':
          result.ssl_root_cert_url = value;
          break;
        case 'SSH_ENABLED':
          result.ssh_enabled = value.toLowerCase() === 'true';
          break;
        case 'SSH_HOST':
          result.ssh_host = value;
          break;
        case 'SSH_PORT':
          result.ssh_port = value;
          break;
        case 'SSH_USERNAME':
          result.ssh_username = value;
          break;
        case 'SSH_PRIVATE_KEY':
          result.ssh_private_key = value;
          break;
        case 'SSH_PASSPHRASE':
          result.ssh_passphrase = value;
          break;
      }
    });
    return result;
  };

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
            onClose();
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
            <div>
              <label className="block font-bold mb-2 text-lg">Paste Credentials</label>
              <p className="text-gray-600 text-sm mb-2">
                Paste your database credentials in the following format:
              </p>
              <textarea
                ref={credentialsTextAreaRef}
                className="neo-input w-full font-mono text-sm"
                defaultValue={credentialsValue}
                placeholder={`DATABASE_TYPE=postgresql
DATABASE_HOST=your-host.example.com
DATABASE_PORT=5432
DATABASE_NAME=your_database
DATABASE_USERNAME=your_username
DATABASE_PASSWORD=your_password
USE_SSL=false
SSL_MODE=disable
SSL_CERT_URL=https://example.com/cert.pem
SSL_KEY_URL=https://example.com/key.pem
SSL_ROOT_CERT_URL=https://example.com/ca.pem
SSH_ENABLED=false
SSH_HOST=ssh.example.com
SSH_PORT=22
SSH_USERNAME=ssh_user
SSH_PRIVATE_KEY=your_private_key`}
                rows={6}
                onChange={(e) => {
                  setCredentialsValue(e.target.value);
                  const parsed = parseConnectionString(e.target.value);
                  setFormData(prev => ({
                    ...prev,
                    ...parsed,
                    // Keep existing password if we're editing and no new password provided
                    password: parsed.password || (initialData ? formData.password : '')
                  }));
                  // Clear any errors for fields that were filled
                  const newErrors = { ...errors };
                  Object.keys(parsed).forEach(key => {
                    delete newErrors[key as keyof FormErrors];
                  });
                  setErrors(newErrors);
                  // Mark fields as touched
                  const newTouched = { ...touched };
                  Object.keys(parsed).forEach(key => {
                    newTouched[key] = true;
                  });
                  setTouched(newTouched);
                  
                  // Set the connection type tab based on SSH enabled
                  if (parsed.ssh_enabled) {
                    handleConnectionTypeChange('ssh');
                  } else {
                    handleConnectionTypeChange('basic');
                  }
                }}
              />
              <p className="text-gray-500 text-xs mt-2">
                All the fields will be automatically filled based on the pasted credentials
              </p>
            </div>
            
            <div className="my-6 border-t border-gray-200"></div>
            
            {/* Connection type tabs */}
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

            {/* Connection Tabs Content */}
            {connectionType === 'basic' ? (
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
        return (
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
            setAutoExecuteQuery={setAutoExecuteQuery}
            setShareWithAI={setShareWithAI}
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
                <h2 className="text-2xl font-bold">{initialData ? 'Edit Connection' : 'New Connection'}</h2>
                <p className="text-gray-500 text-sm">Your database credentials are stored in encrypted form.</p>
              </div>
            </div>
            <button
              onClick={onClose}
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
                <span className="hidden md:block">Schema</span>
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
            {initialData && !successMessage && !isLoading && activeTab === 'connection' && (
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
                    ? (showingNewlyCreatedSchema && activeTab === 'schema') ? 'Save Schema' : 'Create' 
                    : activeTab === 'settings' 
                      ? 'Update Settings' 
                      : activeTab === 'schema' 
                        ? 'Update Schema' 
                        : 'Update Connection'}
                </span>
              )}
            </button>
            <button
              type="button"
              onClick={onClose}
              className="neo-button-secondary flex-1"
              disabled={isLoading}
            >
              Cancel
            </button>
          </div>
          </>
        )}
      </form>
    </div>
  </div>
);
}