import React, { useEffect, useMemo, useRef, useState } from 'react';
import { Chat, ChatSettings, Connection, TableInfo, FileUpload, SSHAuthMethod } from '../types/chat';
import chatService from '../services/chatService';
import { useStream } from '../contexts/StreamContext';
import { FormErrors, validateField, formatConnectionString } from '../utils/connectionValidation';
import { uploadFilesToChat } from '../utils/fileUploadHelper';

// ─── Types ────────────────────────────────────────────────────────────────────

export type ConnectionType = 'basic' | 'ssh';
export type ModalTab = 'connection' | 'schema' | 'settings';

export interface ConnectionModalProps {
  initialData?: Chat;
  initialTab?: ModalTab;
  onClose: (updatedChat?: Chat) => void;
  onEdit?: (data?: Connection, settings?: ChatSettings) => Promise<{ success: boolean; error?: string; updatedChat?: Chat }>;
  onSubmit: (data: Connection, settings: ChatSettings) => Promise<{
    success: boolean;
    error?: string;
    chatId?: string;
    selectedCollections?: string;
  }>;
  onUpdateSelectedCollections?: (chatId: string, selectedCollections: string) => Promise<void>;
  onRefreshSchema?: () => Promise<void>;
}

// ─── Return type ──────────────────────────────────────────────────────────────

export interface UseConnectionModalReturn {
  // State (read)
  activeTab: ModalTab;
  connectionType: ConnectionType;
  fileUploads: FileUpload[];
  isLoadingTables: boolean;
  tables: TableInfo[];
  selectedTables: string[];
  expandedTables: Record<string, boolean>;
  schemaSearchQuery: string;
  selectAllTables: boolean;
  showingNewlyCreatedSchema: boolean;
  newChatId: string | undefined;
  currentChatData: Chat | undefined;
  successMessage: string | null;
  showRefreshSchema: boolean;
  isLoading: boolean;
  formData: Connection;
  errors: FormErrors;
  touched: Record<string, boolean>;
  error: string | null;
  schemaValidationError: string | null;
  autoExecuteQuery: boolean;
  shareWithAI: boolean;
  nonTechMode: boolean;
  autoGenerateVisualization: boolean;
  mongoUriValue: string;
  mongoUriSshValue: string;
  // Refs
  mongoUriInputRef: React.RefObject<HTMLInputElement>;
  mongoUriSshInputRef: React.RefObject<HTMLInputElement>;
  credentialsTextAreaRef: React.RefObject<HTMLTextAreaElement>;
  // Handlers
  handleSubmit: (e: React.FormEvent) => Promise<void>;
  handleChange: (e: React.ChangeEvent<HTMLInputElement | HTMLSelectElement>) => void;
  handleBlur: (e: React.FocusEvent<HTMLInputElement>) => void;
  handleRefreshSchema: () => Promise<void>;
  handleUpdateSettings: () => Promise<void>;
  handleUpdateSchema: () => Promise<void>;
  handleTabChange: (tab: ModalTab) => void;
  handleConnectionTypeChange: (type: ConnectionType) => void;
  toggleTable: (tableName: string) => void;
  toggleExpandTable: (tableName: string, forceState?: boolean) => void;
  toggleSelectAllTables: () => void;
  // Wrapped callbacks for sub-components
  handleFilesChange: (files: FileUpload[]) => void;
  handleGoogleAuthChange: (authData: {
    google_sheet_id: string;
    google_sheet_url: string;
    google_auth_token: string;
    google_refresh_token: string;
  }) => void;
  reloadTables: () => void;
  // Direct setters
  setSchemaSearchQuery: React.Dispatch<React.SetStateAction<string>>;
  setShowRefreshSchema: React.Dispatch<React.SetStateAction<boolean>>;
  setAutoExecuteQuery: React.Dispatch<React.SetStateAction<boolean>>;
  setShareWithAI: React.Dispatch<React.SetStateAction<boolean>>;
  setNonTechMode: React.Dispatch<React.SetStateAction<boolean>>;
  setAutoGenerateVisualization: React.Dispatch<React.SetStateAction<boolean>>;
  setMongoUriValue: React.Dispatch<React.SetStateAction<string>>;
  setMongoUriSshValue: React.Dispatch<React.SetStateAction<string>>;
}

// ─── Hook ─────────────────────────────────────────────────────────────────────

export function useConnectionModal({
  initialData,
  initialTab,
  onClose,
  onEdit,
  onSubmit,
  onUpdateSelectedCollections,
  onRefreshSchema,
}: ConnectionModalProps): UseConnectionModalReturn {
  const { generateStreamId } = useStream();

  // ── Tab state ──────────────────────────────────────────────────────────────
  const [activeTab, setActiveTab] = useState<ModalTab>(initialTab || 'connection');
  const [connectionType, setConnectionType] = useState<ConnectionType>('basic');
  const [prevConnectionType, setPrevConnectionType] = useState<ConnectionType>('basic');
  const [fileUploads, setFileUploads] = useState<FileUpload[]>([]);

  // ── Schema tab state ───────────────────────────────────────────────────────
  const [isLoadingTables, setIsLoadingTables] = useState(false);
  const [tables, setTables] = useState<TableInfo[]>([]);
  const [selectedTables, setSelectedTables] = useState<string[]>([]);
  const [expandedTables, setExpandedTables] = useState<Record<string, boolean>>({});
  const [schemaSearchQuery, setSchemaSearchQuery] = useState('');
  const [selectAllTables, setSelectAllTables] = useState(true);

  // ── New-connection state ───────────────────────────────────────────────────
  const [showingNewlyCreatedSchema, setShowingNewlyCreatedSchema] = useState(false);
  const [newChatId, setNewChatId] = useState<string | undefined>(undefined);
  const [currentChatData, setCurrentChatData] = useState<Chat | undefined>(initialData);

  // ── Feedback state ─────────────────────────────────────────────────────────
  const [successMessage, setSuccessMessage] = useState<string | null>(null);
  const [showRefreshSchema, setShowRefreshSchema] = useState(false);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [schemaValidationError, setSchemaValidationError] = useState<string | null>(null);

  // ── Form state ─────────────────────────────────────────────────────────────
  const [formData, setFormData] = useState<Connection>({
    type: initialData?.connection.type || 'postgresql',
    host: initialData?.connection.host || '',
    port: initialData?.connection.port || '',
    username: initialData?.connection.username || '',
    password: '',
    database: initialData?.connection.database || (initialData?.connection.type === 'spreadsheet' ? 'spreadsheet_db' : ''),
    auth_database: initialData?.connection.auth_database || 'admin',
    use_ssl: initialData?.connection.use_ssl || false,
    ssl_mode: initialData?.connection.ssl_mode || 'disable',
    ssl_cert_url: initialData?.connection.ssl_cert_url || '',
    ssl_key_url: initialData?.connection.ssl_key_url || '',
    ssl_root_cert_url: initialData?.connection.ssl_root_cert_url || '',
    ssh_enabled: initialData?.connection.ssh_enabled || false,
    ssh_host: initialData?.connection.ssh_host || '',
    ssh_port: initialData?.connection.ssh_port || '22',
    ssh_username: initialData?.connection.ssh_username || '',
    ssh_auth_method: initialData?.connection.ssh_auth_method || SSHAuthMethod.PublicKey,
    ssh_private_key: initialData?.connection.ssh_private_key || '',
    ssh_private_key_url: initialData?.connection.ssh_private_key_url || '',
    ssh_passphrase: initialData?.connection.ssh_passphrase || '',
    ssh_password: initialData?.connection.ssh_password || '',
    is_example_db: false,
    google_sheet_id: initialData?.connection.google_sheet_id || '',
    google_sheet_url: initialData?.connection.google_sheet_url || '',
    google_auth_token: initialData?.connection.google_auth_token || '',
    google_refresh_token: initialData?.connection.google_refresh_token || '',
  });
  const [errors, setErrors] = useState<FormErrors>({});
  const [touched, setTouched] = useState<Record<string, boolean>>({});

  // ── Settings state ─────────────────────────────────────────────────────────
  const [autoExecuteQuery, setAutoExecuteQuery] = useState<boolean>(
    initialData?.settings.auto_execute_query !== undefined ? initialData.settings.auto_execute_query : true
  );
  const [shareWithAI, setShareWithAI] = useState<boolean>(
    initialData?.settings.share_data_with_ai !== undefined ? initialData.settings.share_data_with_ai : false
  );
  const [nonTechMode, setNonTechMode] = useState<boolean>(
    initialData?.settings.non_tech_mode !== undefined ? initialData.settings.non_tech_mode : false
  );
  const [autoGenerateVisualization, setAutoGenerateVisualization] = useState<boolean>(
    initialData?.settings.auto_generate_visualization !== undefined
      ? initialData.settings.auto_generate_visualization
      : false
  );

  // ── MongoDB / credentials state ────────────────────────────────────────────
  const [mongoUriValue, setMongoUriValue] = useState<string>('');
  const [mongoUriSshValue, setMongoUriSshValue] = useState<string>('');
  const [credentialsValue, setCredentialsValue] = useState<string>('');

  // ── Refs ───────────────────────────────────────────────────────────────────
  const mongoUriInputRef = useRef<HTMLInputElement>(null);
  const mongoUriSshInputRef = useRef<HTMLInputElement>(null);
  const credentialsTextAreaRef = useRef<HTMLTextAreaElement>(null);
  const isLoadingTablesRef = useRef(false);
  const hasLoadedTablesRef = useRef(false);
  const lastLoadedChatIdRef = useRef<string | undefined>(undefined);

  const [, setTabsVisited] = useState<Record<ModalTab, boolean>>({
    connection: true,
    schema: false,
    settings: false,
  });

  // ── handlers needed before effects ────────────────────────────────────────
  const handleConnectionTypeChange = (type: ConnectionType) => {
    setPrevConnectionType(connectionType);
    setConnectionType(type);
  };

  // ── Effect: sync when initialData changes ──────────────────────────────────
  useEffect(() => {
    const currentChatId = initialData?.id;
    if (currentChatId && currentChatId !== lastLoadedChatIdRef.current) {
      hasLoadedTablesRef.current = false;
      lastLoadedChatIdRef.current = undefined;
    }

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

      if (initialData.connection.ssh_enabled) {
        handleConnectionTypeChange('ssh');
      } else {
        handleConnectionTypeChange('basic');
      }

      const formattedConnectionString = formatConnectionString(initialData.connection);
      setCredentialsValue(formattedConnectionString);
      if (credentialsTextAreaRef.current) {
        credentialsTextAreaRef.current.value = formattedConnectionString;
      }

      if (initialData.connection.type === 'mongodb') {
        const formatMongoURI = (connection: Connection): string => {
          const auth = connection.username
            ? `${connection.username}${connection.password ? `:${connection.password}` : ''}@`
            : '';
          const srv = connection.host.includes('.mongodb.net') ? '+srv' : '';
          const portPart = srv ? '' : `:${connection.port || '27017'}`;
          const dbPart = connection.database ? `/${connection.database}` : '';
          const authSource = connection.auth_database ? `?authSource=${connection.auth_database}` : '';
          return `mongodb${srv}://${auth}${connection.host}${portPart}${dbPart}${authSource}`;
        };

        const mongoUri = formatMongoURI(initialData.connection);
        setMongoUriValue(mongoUri);
        setMongoUriSshValue(mongoUri);
        if (mongoUriInputRef.current) mongoUriInputRef.current.value = mongoUri;
        if (mongoUriSshInputRef.current) mongoUriSshInputRef.current.value = mongoUri;
      }
    }
  }, [initialData]); // eslint-disable-line react-hooks/exhaustive-deps

  // ── Memoized formData.type to stabilise effect deps ───────────────────────
  const formDataType = useMemo(() => formData.type, [formData.type]);

  // ── Ref tracking previous effect deps ────────────────────────────────────
  const prevDepsRef = useRef({
    initialDataId: initialData?.id,
    activeTab,
    newChatId,
    showingNewlyCreatedSchema,
    formDataType,
  });

  // Forward-declare loadTables so the effect below can reference it
  // (defined fully after this effect)
  const loadTablesRef = useRef<(caller?: string) => Promise<void>>(() => Promise.resolve());

  // ── Effect: load tables ───────────────────────────────────────────────────
  useEffect(() => {
    if (isLoadingTablesRef.current) {
      console.log('loadTables useEffect: Already loading, skipping (early exit)');
      return;
    }

    const currentChatId = initialData?.id || (showingNewlyCreatedSchema ? newChatId : undefined);

    const changes = {
      initialDataId: prevDepsRef.current.initialDataId !== initialData?.id,
      activeTab: prevDepsRef.current.activeTab !== activeTab,
      newChatId: prevDepsRef.current.newChatId !== newChatId,
      showingNewlyCreatedSchema: prevDepsRef.current.showingNewlyCreatedSchema !== showingNewlyCreatedSchema,
      formDataType: prevDepsRef.current.formDataType !== formDataType,
    };
    const whatChanged = Object.keys(changes).filter((k) => changes[k as keyof typeof changes]);
    console.log('loadTables useEffect TRIGGERED - WHAT CHANGED:', whatChanged.length > 0 ? whatChanged : ['INITIAL_RENDER']);
    console.log('  └─ Details:', {
      currentChatId,
      activeTab,
      'initialData?.id': initialData?.id,
      newChatId,
      showingNewlyCreatedSchema,
      formDataType,
      isLoadingTablesRef: isLoadingTablesRef.current,
      hasLoadedTablesRef: hasLoadedTablesRef.current,
      lastLoadedChatIdRef: lastLoadedChatIdRef.current,
    });

    prevDepsRef.current = { initialDataId: initialData?.id, activeTab, newChatId, showingNewlyCreatedSchema, formDataType };

    if (hasLoadedTablesRef.current && lastLoadedChatIdRef.current === currentChatId) {
      console.log('loadTables useEffect: Already loaded for this chat, skipping');
      return;
    }
    if (!currentChatId) {
      console.log('loadTables useEffect: No chat ID, skipping');
      return;
    }
    if (tables.length > 0 && lastLoadedChatIdRef.current === currentChatId) {
      console.log('loadTables useEffect: Tables already loaded, skipping');
      return;
    }

    const isSchemaTabActive = activeTab === 'schema';
    const isSpreadsheetOrSheetsInConnectionTab =
      activeTab === 'connection' && (formData.type === 'spreadsheet' || formData.type === 'google_sheets');

    if (!isSchemaTabActive && !isSpreadsheetOrSheetsInConnectionTab) {
      console.log('loadTables useEffect: Not in correct tab, skipping');
      return;
    }

    const hasValidChat = (initialData && initialData.id) || (showingNewlyCreatedSchema && newChatId);
    if (!hasValidChat) {
      console.log('loadTables useEffect: No valid chat, skipping');
      return;
    }

    console.log('loadTables useEffect: All checks passed, loading tables for chat:', currentChatId);
    isLoadingTablesRef.current = true;
    hasLoadedTablesRef.current = true;
    lastLoadedChatIdRef.current = currentChatId;
    loadTablesRef.current();
  }, [initialData?.id, activeTab, newChatId, showingNewlyCreatedSchema, formDataType]); // eslint-disable-line react-hooks/exhaustive-deps

  // ── Effect: sync MongoDB URI inputs on tab change ─────────────────────────
  useEffect(() => {
    if (formData.type === 'mongodb') {
      if (mongoUriInputRef.current && mongoUriValue) mongoUriInputRef.current.value = mongoUriValue;
      if (mongoUriSshInputRef.current && mongoUriSshValue) mongoUriSshInputRef.current.value = mongoUriSshValue;
    }
    if (credentialsTextAreaRef.current && credentialsValue) {
      credentialsTextAreaRef.current.value = credentialsValue;
    }
  }, [activeTab, formData.type, mongoUriValue, mongoUriSshValue, credentialsValue]);

  // ── Effect: persist MongoDB URI when toggling connection type ─────────────
  useEffect(() => {
    if (formData.type === 'mongodb') {
      if (prevConnectionType === 'basic' && connectionType === 'ssh' && mongoUriValue) {
        setMongoUriSshValue(mongoUriValue);
        if (mongoUriSshInputRef.current) mongoUriSshInputRef.current.value = mongoUriValue;
      }
      if (prevConnectionType === 'ssh' && connectionType === 'basic' && mongoUriSshValue) {
        setMongoUriValue(mongoUriSshValue);
        if (mongoUriInputRef.current) mongoUriInputRef.current.value = mongoUriSshValue;
      }
    }
  }, [connectionType, prevConnectionType, formData.type, mongoUriValue, mongoUriSshValue]);

  // ── Effect: auto-dismiss success messages ─────────────────────────────────
  useEffect(() => {
    let timer: ReturnType<typeof setTimeout>;
    if (successMessage) {
      timer = setTimeout(() => setSuccessMessage(null), 3000);
    }
    return () => {
      if (timer) clearTimeout(timer);
    };
  }, [successMessage]);

  // ── loadTables ────────────────────────────────────────────────────────────
  const loadTables = async (caller = 'useEffect') => {
    const chatId = initialData ? initialData.id : showingNewlyCreatedSchema ? newChatId : undefined;
    console.log(`loadTables called from: ${caller}, chatId: ${chatId}`);

    if (!chatId) {
      console.log(`loadTables: No chatId, resetting flags (caller: ${caller})`);
      isLoadingTablesRef.current = false;
      hasLoadedTablesRef.current = false;
      return;
    }

    try {
      setIsLoadingTables(true);
      setError(null);
      setSchemaValidationError(null);

      console.log(`loadTables: About to call chatService.getTables for chatId: ${chatId}`);
      const tablesResponse = await chatService.getTables(chatId);
      setTables(tablesResponse.tables || []);

      const selectedTableNames =
        tablesResponse.tables?.filter((table: TableInfo) => table.is_selected).map((table: TableInfo) => table.name) || [];
      setSelectedTables(selectedTableNames);
      setSelectAllTables(selectedTableNames?.length === tablesResponse.tables?.length);
    } catch (err: any) {
      console.error('Failed to load tables:', err);
      setError(err.message || 'Failed to load tables');
      hasLoadedTablesRef.current = false;
      lastLoadedChatIdRef.current = undefined;
    } finally {
      setIsLoadingTables(false);
      isLoadingTablesRef.current = false;
    }
  };

  // Wire loadTablesRef so the effect can call the closure
  loadTablesRef.current = loadTables;

  // ── handleUpdateSettings ───────────────────────────────────────────────────
  const handleUpdateSettings = async () => {
    if (!initialData || !onEdit) return;
    try {
      setIsLoading(true);
      setError(null);
      setSuccessMessage(null);

      const result = await onEdit(undefined, {
        auto_execute_query: autoExecuteQuery,
        share_data_with_ai: shareWithAI,
        non_tech_mode: nonTechMode,
        auto_generate_visualization: autoGenerateVisualization,
      });

      if (result?.success) {
        if (result.updatedChat) {
          setAutoExecuteQuery(result.updatedChat.settings.auto_execute_query);
          setShareWithAI(result.updatedChat.settings.share_data_with_ai);
          setNonTechMode(result.updatedChat.settings.non_tech_mode);
          setAutoGenerateVisualization(result.updatedChat.settings.auto_generate_visualization);
          setCurrentChatData(result.updatedChat);
        }
        setSuccessMessage('Settings updated successfully');
      } else if (result?.error) {
        setError(result.error);
      }
    } catch (err: any) {
      console.error('Failed to update settings:', err);
      setError(err.message || 'Failed to update settings');
    } finally {
      setIsLoading(false);
    }
  };

  // ── handleSubmit ───────────────────────────────────────────────────────────
  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setIsLoading(true);
    setError(null);
    setSuccessMessage(null);

    if (activeTab === 'schema' && initialData) {
      await handleUpdateSchema();
      return;
    }

    if (activeTab === 'settings' && initialData) {
      await handleUpdateSettings();
      return;
    }

    const updatedFormData = { ...formData, ssh_enabled: connectionType === 'ssh' };
    setFormData(updatedFormData);

    const newErrors: FormErrors = {};
    let hasErrors = false;

    if (updatedFormData.type === 'spreadsheet') {
      if (!updatedFormData.file_uploads || updatedFormData.file_uploads.length === 0) {
        setError('Please upload at least one file');
        setIsLoading(false);
        return;
      }
    } else if (updatedFormData.type === 'google_sheets') {
      if (!updatedFormData.google_sheet_id || !updatedFormData.google_auth_token) {
        setError('Please authenticate with Google and validate a Google Sheets URL');
        setIsLoading(false);
        return;
      }
    } else {
      ['host', 'port', 'database', 'username'].forEach((field) => {
        const fieldError = validateField(field, updatedFormData);
        if (fieldError) {
          newErrors[field as keyof FormErrors] = fieldError;
          hasErrors = true;
        }
      });
    }

    if (connectionType === 'basic' && updatedFormData.use_ssl) {
      if (['verify-ca', 'verify-full'].includes(updatedFormData.ssl_mode || '')) {
        ['ssl_cert_url', 'ssl_key_url', 'ssl_root_cert_url'].forEach((field) => {
          const fieldError = validateField(field, updatedFormData);
          if (fieldError) {
            newErrors[field as keyof FormErrors] = fieldError;
            hasErrors = true;
          }
        });
      }
    }

    if (connectionType === 'ssh') {
      ['ssh_host', 'ssh_port', 'ssh_username'].forEach((field) => {
        const fieldError = validateField(field, updatedFormData);
        if (fieldError) {
          newErrors[field as keyof FormErrors] = fieldError;
          hasErrors = true;
        }
      });
      if (updatedFormData.ssh_auth_method === SSHAuthMethod.PublicKey) {
        const fieldError = validateField('ssh_private_key', updatedFormData);
        if (fieldError) {
          newErrors.ssh_private_key = fieldError;
          hasErrors = true;
        }
      }
    }

    setErrors(newErrors);
    setTouched({
      host: true,
      port: true,
      database: true,
      username: true,
      ...(updatedFormData.use_ssl && connectionType === 'basic'
        ? { ssl_cert_url: true, ssl_key_url: true, ssl_root_cert_url: true }
        : {}),
      ...(connectionType === 'ssh'
        ? { ssh_host: true, ssh_port: true, ssh_username: true, ssh_private_key: true }
        : {}),
    });

    if (hasErrors) {
      setIsLoading(false);
      return;
    }

    try {
      // Block 1: uploading files for a newly-created spreadsheet connection
      if (showingNewlyCreatedSchema && newChatId && formData.type === 'spreadsheet' && fileUploads.length > 0) {
        try {
          setSuccessMessage('Uploading files...');
          await uploadFilesToChat(newChatId, fileUploads);
          setSuccessMessage('Files uploaded successfully. Loading Knowledge Base Tables...');
          await new Promise((resolve) => setTimeout(resolve, 1000));
          setActiveTab('schema');
          setFileUploads([]);
          setFormData((prev) => ({ ...prev, file_uploads: [] }));
          setTables([]);
          hasLoadedTablesRef.current = false;
          await loadTables('handleSubmit-newlyCreated');
          try {
            const updatedChat = await chatService.getChat(newChatId);
            setCurrentChatData(updatedChat);
            setFormData((prev) => ({ ...prev, database: updatedChat.connection.database }));
          } catch (err) {
            console.error('Failed to refresh chat data:', err);
          }
          setSuccessMessage('Files uploaded and tables loaded successfully');
          setIsLoading(false);
          return;
        } catch (err: any) {
          console.error('Failed to upload files:', err);
          setError(err.message || 'Failed to upload files');
          setIsLoading(false);
          return;
        }
      }

      if (initialData) {
        const credentialsChanged =
          initialData.connection.database !== updatedFormData.database ||
          initialData.connection.host !== updatedFormData.host ||
          initialData.connection.port !== updatedFormData.port ||
          initialData.connection.username !== updatedFormData.username;

        const result = await onEdit?.(updatedFormData, {
          auto_execute_query: autoExecuteQuery,
          share_data_with_ai: shareWithAI,
          non_tech_mode: nonTechMode,
          auto_generate_visualization: autoGenerateVisualization,
        });
        console.log('edit result in connection modal', result);

        if (result?.success) {
          if (result.updatedChat) {
            setAutoExecuteQuery(result.updatedChat.settings.auto_execute_query);
            setShareWithAI(result.updatedChat.settings.share_data_with_ai);
            setNonTechMode(result.updatedChat.settings.non_tech_mode);
            setCurrentChatData(result.updatedChat);
            if (result.updatedChat?.connection.type === 'spreadsheet') {
              setFormData((prev) => ({ ...prev, database: result.updatedChat!.connection.database }));
            }
          }

          // Block 2: uploading files for an edited spreadsheet connection
          if (updatedFormData.type === 'spreadsheet' && fileUploads.length > 0) {
            try {
              setSuccessMessage('Uploading files...');
              await uploadFilesToChat(initialData.id, fileUploads);
              setSuccessMessage('Files uploaded successfully. Loading Knowledge Base Tables...');
              setFileUploads([]);
              setFormData((prev) => ({ ...prev, file_uploads: [] }));
              await new Promise((resolve) => setTimeout(resolve, 1000));
              setActiveTab('schema');
              setTables([]);
              hasLoadedTablesRef.current = false;
              await loadTables('handleSubmit-edit');
              try {
                const updatedChat = await chatService.getChat(initialData.id);
                setCurrentChatData(updatedChat);
                setFormData((prev) => ({ ...prev, database: updatedChat.connection.database }));
              } catch (err) {
                console.error('Failed to refresh chat data:', err);
              }
              setSuccessMessage('Files uploaded and tables loaded successfully');
              setIsLoading(false);
            } catch (err: any) {
              console.error('Failed to upload files:', err);
              setError(err.message || 'Failed to upload files');
              setIsLoading(false);
              return;
            }
          } else if (credentialsChanged && activeTab === 'connection') {
            setActiveTab('schema');
            hasLoadedTablesRef.current = false;
            loadTables('handleSubmit-credentialsChanged');
            setIsLoading(false);
          } else {
            setSuccessMessage('Connection updated successfully');
            setIsLoading(false);
          }
        } else if (result?.error) {
          setError(result.error);
          setIsLoading(false);
        }
      } else {
        // New connection
        const result = await onSubmit(updatedFormData, {
          auto_execute_query: autoExecuteQuery,
          share_data_with_ai: shareWithAI,
          non_tech_mode: nonTechMode,
          auto_generate_visualization: autoGenerateVisualization,
        });
        console.log('submit result in connection modal', result);

        if (result?.success) {
          if (result.chatId) {
            setNewChatId(result.chatId);
            setShowingNewlyCreatedSchema(true);

            // Block 3: uploading files for a brand-new spreadsheet/Google Sheets connection
            if ((updatedFormData.type === 'spreadsheet' && fileUploads.length > 0) || updatedFormData.type === 'google_sheets') {
              try {
                setSuccessMessage('Establishing connection...');
                await chatService.connectToConnection(result.chatId, generateStreamId());
                await new Promise((resolve) => setTimeout(resolve, 500));

                if (updatedFormData.type === 'spreadsheet' && fileUploads.length > 0) {
                  setSuccessMessage('Uploading files...');
                  await uploadFilesToChat(result.chatId, fileUploads);
                  setSuccessMessage('Files uploaded successfully. Loading Knowledge Base Tables...');
                } else if (updatedFormData.type === 'google_sheets') {
                  setSuccessMessage('Syncing Google Sheets data...');
                }
              } catch (err: any) {
                console.error('Failed to upload files:', err);
                setError(err.message || 'Failed to upload files');
                setIsLoading(false);
                return;
              }
            }

            setActiveTab('schema');
            hasLoadedTablesRef.current = false;
            setIsLoadingTables(true);

            try {
              console.log('ConnectionModal: Loading tables for new connection, chatId:', result.chatId);
              const tablesResponse = await chatService.getTables(result.chatId);
              setTables(tablesResponse.tables || []);
              const selectedTableNames =
                tablesResponse.tables?.filter((t: TableInfo) => t.is_selected).map((t: TableInfo) => t.name) || [];
              setSelectedTables(selectedTableNames);
              setSelectAllTables(selectedTableNames?.length === tablesResponse.tables?.length);

              if (updatedFormData.type === 'spreadsheet') {
                try {
                  const updatedChat = await chatService.getChat(result.chatId);
                  setCurrentChatData(updatedChat);
                  setFormData((prev) => ({ ...prev, database: updatedChat.connection.database }));
                } catch (err) {
                  console.error('Failed to refresh chat data:', err);
                }
              }

              console.log('Connection created. Now you can select tables to include in your schema.');
              setSuccessMessage(
                updatedFormData.type === 'spreadsheet'
                  ? 'Files uploaded successfully. Review your Knowledge Base Tables.'
                  : 'Connection created successfully. Select tables to include in your schema.'
              );
            } catch (err: any) {
              console.error('Failed to load tables for new connection:', err);
              setError(err.message || 'Failed to load tables for new connection');
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

  // ── handleChange ───────────────────────────────────────────────────────────
  const handleChange = (e: React.ChangeEvent<HTMLInputElement | HTMLSelectElement>) => {
    const { name, value } = e.target;

    if (name === 'type' && value === 'spreadsheet') {
      setFormData((prev) => ({
        ...prev,
        [name]: value,
        database: currentChatData?.connection.database || 'spreadsheet_db',
        host: 'internal-spreadsheet',
        port: '0',
        username: 'spreadsheet_user',
        password: 'internal',
      }));
    } else {
      setFormData((prev) => ({ ...prev, [name]: value }));
    }

    if (touched[name]) {
      const fieldError = validateField(name, { ...formData, [name]: value });
      setErrors((prev) => ({ ...prev, [name]: fieldError }));
    }
  };

  // ── handleBlur ─────────────────────────────────────────────────────────────
  const handleBlur = (e: React.FocusEvent<HTMLInputElement>) => {
    const { name } = e.target;
    setTouched((prev) => ({ ...prev, [name]: true }));
    const fieldError = validateField(name, formData);
    setErrors((prev) => ({ ...prev, [name]: fieldError }));
  };

  // ── handleRefreshSchema ────────────────────────────────────────────────────
  const handleRefreshSchema = async () => {
    if (!onRefreshSchema) return;
    try {
      await onRefreshSchema();
      setShowRefreshSchema(false);
    } catch (err) {
      console.error('Failed to refresh schema:', err);
    }
  };

  // ── toggleTable ────────────────────────────────────────────────────────────
  const toggleTable = (tableName: string) => {
    setSchemaValidationError(null);
    setSelectedTables((prev) => {
      if (prev.includes(tableName)) {
        setSelectAllTables(false);
        if (prev.length === 1) {
          setSchemaValidationError('At least one table must be selected');
          return prev;
        }
        return prev.filter((n) => n !== tableName);
      } else {
        const newSelected = [...prev, tableName];
        if (newSelected.length === tables?.length) setSelectAllTables(true);
        return newSelected;
      }
    });
  };

  // ── toggleExpandTable ──────────────────────────────────────────────────────
  const toggleExpandTable = (tableName: string, forceState?: boolean) => {
    if (tableName === '') {
      const allExpanded = Object.values(expandedTables).every((v) => v);
      const newState = forceState !== undefined ? forceState : !allExpanded;
      setExpandedTables(
        tables.reduce((acc, table) => {
          acc[table.name] = newState;
          return acc;
        }, {} as Record<string, boolean>)
      );
    } else {
      setExpandedTables((prev) => ({
        ...prev,
        [tableName]: forceState !== undefined ? forceState : !prev[tableName],
      }));
    }
  };

  // ── toggleSelectAllTables ──────────────────────────────────────────────────
  const toggleSelectAllTables = () => {
    setSchemaValidationError(null);
    if (selectAllTables) {
      setSchemaValidationError('At least one table must be selected');
      return;
    }
    setSelectedTables(tables?.map((table) => table.name) || []);
    setSelectAllTables(true);
  };

  // ── handleUpdateSchema ─────────────────────────────────────────────────────
  const handleUpdateSchema = async () => {
    if (!initialData && !showingNewlyCreatedSchema) return;

    if (formData.type === 'spreadsheet' || formData.type === 'google_sheets') {
      setSuccessMessage('Knowledge Base saved successfully');
      setTimeout(() => onClose(currentChatData), 500);
      return;
    }

    if (selectedTables?.length === 0) {
      setSchemaValidationError('At least one table must be selected');
      return;
    }

    try {
      setIsLoading(true);
      setError(null);
      setSchemaValidationError(null);
      setSuccessMessage(null);

      const formattedSelection = selectAllTables ? 'ALL' : selectedTables.join(',');
      const chatId = showingNewlyCreatedSchema ? newChatId : initialData!.id;

      if (onUpdateSelectedCollections && chatId) {
        await onUpdateSelectedCollections(chatId, formattedSelection);
        setSuccessMessage('Knowledge Base Tables updated successfully');

        if (!initialData && showingNewlyCreatedSchema) {
          setTimeout(() => onClose(currentChatData), 1000);
        }

        console.log('Schema selection updated successfully');
      }
    } catch (err: any) {
      console.error('Failed to update selected tables:', err);
      setError(err.message || 'Failed to update selected tables');
    } finally {
      setIsLoading(false);
    }
  };

  // ── handleTabChange ────────────────────────────────────────────────────────
  const handleTabChange = (tab: ModalTab) => {
    setTabsVisited((prev) => ({ ...prev, [tab]: true }));
    setActiveTab(tab);
  };

  // ── Wrapped sub-component callbacks ───────────────────────────────────────
  const handleFilesChange = (files: FileUpload[]) => {
    setFileUploads(files);
    setFormData((prev) => ({ ...prev, file_uploads: files }));
  };

  const handleGoogleAuthChange = (authData: {
    google_sheet_id: string;
    google_sheet_url: string;
    google_auth_token: string;
    google_refresh_token: string;
  }) => {
    setFormData((prev) => ({
      ...prev,
      google_sheet_id: authData.google_sheet_id,
      google_sheet_url: authData.google_sheet_url,
      google_auth_token: authData.google_auth_token,
      google_refresh_token: authData.google_refresh_token,
    }));
  };

  const reloadTables = () => {
    hasLoadedTablesRef.current = false;
    loadTables('manual-reload');
  };

  // ── Return ─────────────────────────────────────────────────────────────────
  return {
    activeTab,
    connectionType,
    fileUploads,
    isLoadingTables,
    tables,
    selectedTables,
    expandedTables,
    schemaSearchQuery,
    selectAllTables,
    showingNewlyCreatedSchema,
    newChatId,
    currentChatData,
    successMessage,
    showRefreshSchema,
    isLoading,
    formData,
    errors,
    touched,
    error,
    schemaValidationError,
    autoExecuteQuery,
    shareWithAI,
    nonTechMode,
    autoGenerateVisualization,
    mongoUriValue,
    mongoUriSshValue,
    mongoUriInputRef,
    mongoUriSshInputRef,
    credentialsTextAreaRef,
    handleSubmit,
    handleChange,
    handleBlur,
    handleRefreshSchema,
    handleUpdateSettings,
    handleUpdateSchema,
    handleTabChange,
    handleConnectionTypeChange,
    toggleTable,
    toggleExpandTable,
    toggleSelectAllTables,
    handleFilesChange,
    handleGoogleAuthChange,
    reloadTables,
    setSchemaSearchQuery,
    setShowRefreshSchema,
    setAutoExecuteQuery,
    setShareWithAI,
    setNonTechMode,
    setAutoGenerateVisualization,
    setMongoUriValue,
    setMongoUriSshValue,
  };
}
