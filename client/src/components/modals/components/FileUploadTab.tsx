import React, { useState, useRef, useEffect } from 'react';
import { Upload, X, FileText, Table, AlertCircle, Info, Loader2, Database } from 'lucide-react';
import { Connection, FileUpload } from '../../../types/chat';

interface FileUploadTabProps {
  formData: Connection;
  handleChange: (e: React.ChangeEvent<HTMLInputElement | HTMLSelectElement>) => void;
  onFilesChange: (files: FileUpload[]) => void;
  isEditMode?: boolean;
  chatId?: string;
  preloadedTables?: any[];
}

const FileUploadTab: React.FC<FileUploadTabProps> = ({
  formData,
  handleChange,
  onFilesChange,
  isEditMode = false,
  chatId,
  preloadedTables,
}) => {
  const [uploadedFiles, setUploadedFiles] = useState<FileUpload[]>(formData.file_uploads || []);
  const [isDragging, setIsDragging] = useState(false);
  const [uploadError, setUploadError] = useState<string | null>(null);
  const [isProcessing, setIsProcessing] = useState(false);
  const [existingTables, setExistingTables] = useState<any[]>(preloadedTables || []);
  const [loadingTables, setLoadingTables] = useState(false);
  const [conflictingTables, setConflictingTables] = useState<string[]>([]);
  const fileInputRef = useRef<HTMLInputElement>(null);

  // Load existing tables when in edit mode
  useEffect(() => {
    if (isEditMode && chatId) {
      loadExistingTables();
    }
  }, [isEditMode, chatId]);
  
  // Reload tables when component becomes visible or preloadedTables change
  useEffect(() => {
    if (preloadedTables && preloadedTables.length > 0) {
      setExistingTables(preloadedTables);
    } else if (isEditMode && chatId && existingTables.length === 0 && !loadingTables) {
      loadExistingTables();
    }
  }, [isEditMode, chatId, existingTables.length, loadingTables, preloadedTables]);
  
  // Clear uploaded files when entering edit mode
  useEffect(() => {
    if (isEditMode) {
      setUploadedFiles([]);
      setUploadError(null);
    }
  }, [isEditMode]);

  const loadExistingTables = async () => {
    if (!chatId) return;
    
    setLoadingTables(true);
    try {
      const response = await fetch(`${import.meta.env.VITE_API_URL}/chats/${chatId}/tables`, {
        headers: {
          'Authorization': `Bearer ${localStorage.getItem('token')}`
        }
      });
      if (response.ok) {
        const data = await response.json();
        setExistingTables(data.data?.tables || []);
      }
    } catch (error) {
      console.error('Failed to load existing tables:', error);
    } finally {
      setLoadingTables(false);
    }
  };

  const handleDragOver = (e: React.DragEvent) => {
    e.preventDefault();
    setIsDragging(true);
  };

  const handleDragLeave = (e: React.DragEvent) => {
    e.preventDefault();
    setIsDragging(false);
  };

  const handleDrop = (e: React.DragEvent) => {
    e.preventDefault();
    setIsDragging(false);
    
    const files = Array.from(e.dataTransfer.files);
    handleFiles(files);
  };

  const handleFileInput = (e: React.ChangeEvent<HTMLInputElement>) => {
    if (e.target.files) {
      const files = Array.from(e.target.files);
      handleFiles(files);
    }
  };

  const handleFiles = async (files: File[]) => {
    setUploadError(null);
    setIsProcessing(true);

    const validExtensions = ['.csv', '.xlsx', '.xls'];
    const maxFileSize = 100 * 1024 * 1024; // 100MB

    const newFiles: FileUpload[] = [];

    for (const file of files) {
      const extension = file.name.substring(file.name.lastIndexOf('.')).toLowerCase();
      
      if (!validExtensions.includes(extension)) {
        setUploadError(`Invalid file type: ${file.name}. Only CSV and Excel files are allowed.`);
        continue;
      }

      if (file.size > maxFileSize) {
        setUploadError(`File too large: ${file.name}. Maximum file size is 100MB.`);
        continue;
      }

      // Check for duplicate files
      if (uploadedFiles.some(f => f.filename === file.name && f.size === file.size)) {
        setUploadError(`Duplicate file: ${file.name} is already uploaded.`);
        continue;
      }

      // Generate table name
      const tableName = sanitizeTableName(file.name);

      // Check if we already have a file with the same table name in current batch or already uploaded
      const existingFileWithSameTable = [...uploadedFiles, ...newFiles].find(f => f.tableName === tableName);
      if (existingFileWithSameTable) {
        setUploadError(`Multiple files map to the same table name "${tableName}". Please rename one of the files: ${existingFileWithSameTable.filename} or ${file.name}`);
        continue;
      }

      const fileUpload: FileUpload = {
        id: `file-${Date.now()}-${Math.random().toString(36).substring(2, 11)}`,
        filename: file.name,
        size: file.size,
        type: extension === '.csv' ? 'csv' : 'excel',
        uploadedAt: new Date(),
        tableName: tableName,
        file: file,
      };

      // In edit mode, track which tables will be replaced
      if (isEditMode && existingTables.some(t => t.name === tableName)) {
        // Always replace existing tables with the same name
        fileUpload.mergeStrategy = 'replace';
        if (!conflictingTables.includes(tableName)) {
          setConflictingTables(prev => [...prev, tableName]);
        }
      }
      
      newFiles.push(fileUpload);
    }

    // Add all files
    if (newFiles.length > 0) {
      const updatedFiles = [...uploadedFiles, ...newFiles];
      setUploadedFiles(updatedFiles);
      onFilesChange(updatedFiles);
      
      // Show info message if tables will be replaced
      if (conflictingTables.length > 0 && isEditMode) {
        setUploadError(`Note: The following tables will be replaced with new data: ${conflictingTables.join(', ')}`);
        setConflictingTables([]);
      }
    }

    setIsProcessing(false);
  };

  const sanitizeTableName = (filename: string): string => {
    // Remove file extension and special characters, replace with underscores
    return filename
      .replace(/\.[^/.]+$/, '') // Remove extension
      .replace(/[^a-zA-Z0-9_]/g, '_') // Replace special chars with underscore
      .replace(/_+/g, '_') // Replace multiple underscores with single
      .replace(/^_|_$/g, '') // Remove leading/trailing underscores
      .toLowerCase();
  };

  const removeFile = (fileId: string) => {
    const updatedFiles = uploadedFiles.filter(f => f.id !== fileId);
    setUploadedFiles(updatedFiles);
    onFilesChange(updatedFiles);
    setUploadError(null); // Clear any info messages when removing files
  };

  const updateTableName = (fileId: string, newTableName: string) => {
    const updatedFiles = uploadedFiles.map(f => 
      f.id === fileId ? { ...f, tableName: newTableName } : f
    );
    setUploadedFiles(updatedFiles);
    onFilesChange(updatedFiles);
  };

  const formatFileSize = (bytes: number): string => {
    if (bytes < 1024) return bytes + ' B';
    if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB';
    return (bytes / (1024 * 1024)).toFixed(1) + ' MB';
  };


  return (
    <>
      {/* Existing Tables Section (Edit Mode Only) */}
      {isEditMode && (
        <div className="mb-6">
          <h3 className="font-bold text-lg mb-3">Existing Data Tables</h3>
          {loadingTables ? (
            <div className="flex items-center justify-center py-8">
              <Loader2 className="w-6 h-6 animate-spin text-gray-500" />
            </div>
          ) : existingTables.length > 0 ? (
            <div className="space-y-2 mb-4">
              {existingTables.map((table) => (
                <div key={table.name} className="p-3 border-2 border-gray-200 rounded-lg bg-gray-50">
                  <div className="flex items-center justify-between">
                    <div className="flex items-center gap-2">
                      <Database className="w-4 h-4 text-gray-600" />
                      <span className="font-medium">{table.name}</span>
                      <span className="text-sm text-gray-500">
                        ({table.row_count || 0} rows • {table.columns?.length || 0} columns)
                      </span>
                    </div>
                    {/* <span className="text-xs text-gray-500">
                      From: {table.source_file || 'Unknown source'}
                    </span> */}
                  </div>
                </div>
              ))}
            </div>
          ) : (
            <p className="text-gray-500 mb-4">No existing tables found.</p>
          )}
          
          <div className="p-4 bg-blue-50 border-2 border-blue-200 rounded-lg mb-6">
            <div className="flex items-start gap-2">
              <Info className="w-5 h-5 text-blue-600 mt-0.5 flex-shrink-0" />
              <div className="text-sm text-blue-800">
                <p className="font-medium mb-1">Uploading Additional Data</p>
                <p>When uploading files:</p>
                <ul className="mt-2 space-y-1 ml-4">
                  <li>• <strong>Existing tables:</strong> Will be replaced entirely with the new file data</li>
                  <li>• <strong>New files:</strong> Will be added as new tables</li>
                  <li>• <strong>Important:</strong> Always upload files containing all data you want to keep</li>
                </ul>
              </div>
            </div>
          </div>
        </div>
      )}

      {/* File Naming Guidelines */}
      <div className="mb-6 p-4 bg-green-50 border-2 border-green-200 rounded-lg">
        <div className="flex items-start gap-2">
          <Info className="w-5 h-5 text-green-600 mt-0.5 flex-shrink-0" />
          <div>
            <h4 className="font-bold text-green-900 mb-2">File Naming Guidelines</h4>
            <ul className="text-sm text-green-800 space-y-2">
              <li><strong>1. Related data:</strong> Use prefixes to group related files (e.g., sales_2024_q1.csv, sales_2024_q2.csv, sales is the table name)</li>
              <li><strong>2. Different datasets:</strong> Use distinct names for unrelated data (e.g., customers.csv, inventory.xlsx, customers & inventory are the table names)</li>
              <li><strong>3. Multi-sheet Excel:</strong> Each sheet will be imported as a separate table with sheet name as suffix (e.g: customer.csv, payments.csv, here customer & payments will be the table names)</li>
              <li><strong>4. Data sorting:</strong>We automatically sort your data based on the column names and date, so please include a date field in your data for better sorting (e.g: created_at, updated_at).</li>
            </ul>
          </div>
        </div>
      </div>

      {/* Hidden Connection Fields for Spreadsheet */}
      <input type="hidden" name="host" value="internal-spreadsheet" onChange={handleChange} />
      <input type="hidden" name="port" value="0" onChange={handleChange} />
      <input type="hidden" name="database" value={formData.database || "spreadsheet_db"} onChange={handleChange} />
      <input type="hidden" name="username" value="spreadsheet_user" onChange={handleChange} />
      <input type="hidden" name="password" value="internal" onChange={handleChange} />

      {/* File Upload Area */}
      <div className="mb-6">
        <label className="block font-bold mb-2 text-lg">
          {isEditMode ? 'Upload More Files' : 'Upload CSV/Excel Files'}
        </label>
        <p className="text-gray-600 text-sm mb-4">
          Upload your CSV or Excel files. They will be securely stored and encrypted in our database.
        </p>

        <div
          className={`border-2 border-dashed rounded-lg p-8 text-center transition-colors ${
            isDragging 
              ? 'border-blue-500 bg-blue-50' 
              : 'border-gray-300 hover:border-gray-400'
          }`}
          onDragOver={handleDragOver}
          onDragLeave={handleDragLeave}
          onDrop={handleDrop}
        >
          <Upload className="w-12 h-12 mx-auto mb-4 text-gray-400" />
          <p className="text-gray-600 mb-4">
            Drag and drop your CSV or Excel files here, or click to browse
          </p>
          <button
            type="button"
            onClick={() => fileInputRef.current?.click()}
            className="neo-button-secondary"
            disabled={isProcessing}
          >
            {isProcessing ? (
              <div className="flex items-center gap-2">
                <Loader2 className="w-4 h-4 animate-spin" />
                <span>Processing...</span>
              </div>
            ) : (
              'Choose Files'
            )}
          </button>
          <input
            ref={fileInputRef}
            type="file"
            multiple
            accept=".csv,.xlsx,.xls"
            onChange={handleFileInput}
            className="hidden"
          />
          <p className="text-xs text-gray-500 mt-4">
            Supported formats: CSV, XLSX, XLS (Max 100MB per file)
          </p>
        </div>

        {uploadError && (
          <div className="mt-2 p-3 bg-red-50 border-2 border-red-200 rounded-lg">
            <div className="flex items-center gap-2 text-red-600">
              <AlertCircle className="w-4 h-4" />
              <p className="text-sm">{uploadError}</p>
            </div>
          </div>
        )}
      </div>

      {/* Uploaded Files List */}
      {uploadedFiles.length > 0 && (
        <div className="mb-6">
          <h4 className="font-bold mb-3 text-lg">Uploaded Files ({uploadedFiles.length})</h4>
          <div className="space-y-3">
            {uploadedFiles.map((file) => (
              <div key={file.id} className="p-4 border-2 border-gray-200 rounded-lg bg-gray-50">
                <div className="flex items-start justify-between mb-2">
                  <div className="flex items-center gap-2">
                    {file.type === 'csv' ? (
                      <FileText className="w-5 h-5 text-green-600" />
                    ) : (
                      <Table className="w-5 h-5 text-blue-600" />
                    )}
                    <div>
                      <p className="font-medium">{file.filename}</p>
                      <p className="text-sm text-gray-500">
                        {formatFileSize(file.size)} • {file.type.toUpperCase()}
                      </p>
                    </div>
                  </div>
                  <button
                    type="button"
                    onClick={() => removeFile(file.id)}
                    className="p-1 hover:bg-gray-200 rounded transition-colors"
                  >
                    <X className="w-4 h-4" />
                  </button>
                </div>
                
                <div className="mt-2">
                  <label className="block text-sm font-medium text-gray-700 mb-1">
                    Table Name
                  </label>
                  <input
                    type="text"
                    value={file.tableName || ''}
                    onChange={(e) => updateTableName(file.id, e.target.value)}
                    className="neo-input w-full text-sm"
                    placeholder="Enter table name"
                  />
                  <p className="text-xs text-gray-500 mt-3">
                    This will be the table name in your database
                  </p>
                </div>
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Data Privacy Notice */}
      <div className="p-4 bg-gray-100 border border-gray-300 rounded-lg">
        <div className="flex items-start gap-2">
          <AlertCircle className="w-5 h-5 text-gray-600 mt-0.5 flex-shrink-0" />
          <div className="text-sm text-gray-700">
            <p className="font-semibold text-base mb-1 mt-0.5">Data Security & Privacy</p>
            <p>
              Your CSV and Excel data will be encrypted using AES-GCM encryption before storage. 
              Each user's data is isolated in separate database schemas. You can export or delete 
              your data at any time.
            </p>
          </div>
        </div>
      </div>

    </>
  );
};

export default FileUploadTab;