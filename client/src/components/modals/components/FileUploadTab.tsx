import React, { useState, useRef, useEffect } from 'react';
import { Upload, X, FileText, Table, AlertCircle, Info, Loader2, Database, RefreshCw, FilePlus, Settings, GitMerge } from 'lucide-react';
import { Connection, FileUpload } from '../../../types/chat';

interface FileUploadTabProps {
  formData: Connection;
  handleChange: (e: React.ChangeEvent<HTMLInputElement | HTMLSelectElement>) => void;
  onFilesChange: (files: FileUpload[]) => void;
  isEditMode?: boolean;
  chatId?: string;
}

const FileUploadTab: React.FC<FileUploadTabProps> = ({
  formData,
  handleChange,
  onFilesChange,
  isEditMode = false,
  chatId,
}) => {
  const [uploadedFiles, setUploadedFiles] = useState<FileUpload[]>(formData.file_uploads || []);
  const [isDragging, setIsDragging] = useState(false);
  const [uploadError, setUploadError] = useState<string | null>(null);
  const [isProcessing, setIsProcessing] = useState(false);
  const [existingTables, setExistingTables] = useState<any[]>([]);
  const [loadingTables, setLoadingTables] = useState(false);
  const [conflictResolution, setConflictResolution] = useState<Record<string, 'replace' | 'append' | 'merge'>>({});
  const [showConflictModal, setShowConflictModal] = useState(false);
  const [conflictingFile, setConflictingFile] = useState<FileUpload | null>(null);
  const [showAdvancedOptions, setShowAdvancedOptions] = useState(false);
  const [selectedStrategy, setSelectedStrategy] = useState<'replace' | 'append' | 'merge' | 'smart_merge'>('replace');
  const [mergeOptions, setMergeOptions] = useState({
    ignoreCase: true,
    trimWhitespace: true,
    handleNulls: 'empty' as const,
    addNewColumns: true,
    dropMissingColumns: false,
    updateExisting: true,
    insertNew: true,
    deleteMissing: false,
  });
  const fileInputRef = useRef<HTMLInputElement>(null);

  // Load existing tables when in edit mode
  useEffect(() => {
    if (isEditMode && chatId) {
      loadExistingTables();
    }
  }, [isEditMode, chatId]);

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

      // Check for conflicts with existing tables in edit mode
      if (isEditMode && existingTables.some(t => t.name === tableName)) {
        // Store the file temporarily and show conflict resolution modal
        const fileUpload: FileUpload = {
          id: `file-${Date.now()}-${Math.random().toString(36).substr(2, 9)}`,
          filename: file.name,
          size: file.size,
          type: extension === '.csv' ? 'csv' : 'excel',
          uploadedAt: new Date(),
          tableName: tableName,
          file: file,
        };
        setConflictingFile(fileUpload);
        setShowConflictModal(true);
        continue;
      }

      const fileUpload: FileUpload = {
        id: `file-${Date.now()}-${Math.random().toString(36).substr(2, 9)}`,
        filename: file.name,
        size: file.size,
        type: extension === '.csv' ? 'csv' : 'excel',
        uploadedAt: new Date(),
        tableName: sanitizeTableName(file.name),
        file: file, // Store the actual File object
      };

      newFiles.push(fileUpload);
    }

    if (newFiles.length > 0) {
      const updatedFiles = [...uploadedFiles, ...newFiles];
      setUploadedFiles(updatedFiles);
      onFilesChange(updatedFiles);
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

  const handleConflictResolution = (resolution: 'replace' | 'append' | 'merge' | 'smart_merge') => {
    if (resolution === 'smart_merge') {
      setSelectedStrategy(resolution);
      setShowAdvancedOptions(true);
    } else {
      if (conflictingFile) {
        // Add the file with the selected resolution strategy
        const updatedFiles = [...uploadedFiles, { 
          ...conflictingFile, 
          mergeStrategy: resolution,
          mergeOptions: resolution === 'merge' ? mergeOptions : undefined
        }];
        setUploadedFiles(updatedFiles);
        onFilesChange(updatedFiles);
        setConflictResolution({ ...conflictResolution, [conflictingFile.tableName || '']: resolution });
      }
      setShowConflictModal(false);
      setConflictingFile(null);
    }
  };

  const handleAdvancedMergeConfirm = () => {
    if (conflictingFile) {
      const updatedFiles = [...uploadedFiles, { 
        ...conflictingFile, 
        mergeStrategy: selectedStrategy,
        mergeOptions: mergeOptions
      }];
      setUploadedFiles(updatedFiles);
      onFilesChange(updatedFiles);
      setConflictResolution({ ...conflictResolution, [conflictingFile.tableName || '']: selectedStrategy });
    }
    setShowAdvancedOptions(false);
    setShowConflictModal(false);
    setConflictingFile(null);
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
                    <span className="text-xs text-gray-500">
                      From: {table.source_file || 'Unknown source'}
                    </span>
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
                <p>When uploading files with the same name as existing tables, you'll be asked how to handle the data:</p>
                <ul className="mt-2 space-y-1 ml-4">
                  <li>• <strong>Replace:</strong> Delete existing data and replace with new file</li>
                  <li>• <strong>Append:</strong> Add new rows to existing table (columns must match)</li>
                  <li>• <strong>Merge:</strong> Smart merge based on common columns (useful for updates)</li>
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
      <input type="hidden" name="database" value="spreadsheet_data" onChange={handleChange} />
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
            <p className="font-medium mb-1 mt-0.5">Data Security & Privacy</p>
            <p>
              Your CSV and Excel data will be encrypted using AES-GCM encryption before storage. 
              Each user's data is isolated in separate database schemas. You can export or delete 
              your data at any time.
            </p>
          </div>
        </div>
      </div>

      {/* Conflict Resolution Modal */}
      {showConflictModal && conflictingFile && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black bg-opacity-50">
          <div className="bg-white rounded-lg p-6 max-w-md w-full mx-4 border-4 border-black shadow-[8px_8px_0px_0px_rgba(0,0,0,1)]">
            <h3 className="text-xl font-bold mb-4">Table Already Exists</h3>
            <p className="mb-4">
              The table <strong>"{conflictingFile.tableName}"</strong> already exists. 
              How would you like to handle the new data from <strong>{conflictingFile.filename}</strong>?
            </p>
            
            <div className="space-y-3 mb-6">
              <button
                onClick={() => handleConflictResolution('replace')}
                className="w-full p-4 border-2 border-gray-300 rounded-lg hover:border-blue-500 hover:bg-blue-50 text-left transition-colors"
              >
                <div className="flex items-start gap-3">
                  <RefreshCw className="w-5 h-5 text-blue-600 mt-0.5" />
                  <div>
                    <p className="font-semibold">Replace Entire Table</p>
                    <p className="text-sm text-gray-600">Delete all existing data and replace with new file</p>
                  </div>
                </div>
              </button>
              
              <button
                onClick={() => handleConflictResolution('append')}
                className="w-full p-4 border-2 border-gray-300 rounded-lg hover:border-green-500 hover:bg-green-50 text-left transition-colors"
              >
                <div className="flex items-start gap-3">
                  <FilePlus className="w-5 h-5 text-green-600 mt-0.5" />
                  <div>
                    <p className="font-semibold">Append New Rows</p>
                    <p className="text-sm text-gray-600">Add new rows to the end of existing table</p>
                  </div>
                </div>
              </button>
              
              <button
                onClick={() => handleConflictResolution('merge')}
                className="w-full p-4 border-2 border-gray-300 rounded-lg hover:border-purple-500 hover:bg-purple-50 text-left transition-colors"
              >
                <div className="flex items-start gap-3">
                  <GitMerge className="w-5 h-5 text-purple-600 mt-0.5" />
                  <div>
                    <p className="font-semibold">Simple Merge</p>
                    <p className="text-sm text-gray-600">Add new rows and update existing ones if all columns match</p>
                  </div>
                </div>
              </button>
              
              <button
                onClick={() => handleConflictResolution('smart_merge')}
                className="w-full p-4 border-2 border-gray-300 rounded-lg hover:border-indigo-500 hover:bg-indigo-50 text-left transition-colors"
              >
                <div className="flex items-start gap-3">
                  <Settings className="w-5 h-5 text-indigo-600 mt-0.5" />
                  <div>
                    <p className="font-semibold">Advanced Merge</p>
                    <p className="text-sm text-gray-600">Configure how to handle column changes, updates, and conflicts</p>
                  </div>
                </div>
              </button>
            </div>
            
            <button
              onClick={() => {
                setShowConflictModal(false);
                setConflictingFile(null);
              }}
              className="w-full neo-button-secondary"
            >
              Cancel
            </button>
          </div>
        </div>
      )}

      {/* Advanced Merge Options Modal */}
      {showAdvancedOptions && conflictingFile && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black bg-opacity-50 overflow-y-auto">
          <div className="bg-white rounded-lg p-6 max-w-2xl w-full mx-4 my-8 border-4 border-black shadow-[8px_8px_0px_0px_rgba(0,0,0,1)] max-h-[90vh] overflow-y-auto">
            <h3 className="text-xl font-bold mb-4">Advanced Merge Options</h3>
            <p className="mb-6 text-gray-600">
              Configure how to merge <strong>{conflictingFile.filename}</strong> into <strong>"{conflictingFile.tableName}"</strong>
            </p>
            
            <div className="space-y-6">
              {/* Column Handling */}
              <div className="border-2 border-gray-200 rounded-lg p-4">
                <h4 className="font-semibold mb-3 flex items-center gap-2">
                  <Table className="w-4 h-4" />
                  Column Handling
                </h4>
                <div className="space-y-3">
                  <label className="flex items-start gap-3">
                    <input
                      type="checkbox"
                      checked={mergeOptions.addNewColumns}
                      onChange={(e) => setMergeOptions({...mergeOptions, addNewColumns: e.target.checked})}
                      className="mt-1"
                    />
                    <div>
                      <p className="font-medium">Add New Columns</p>
                      <p className="text-sm text-gray-600">Add columns that exist in the new file but not in the table</p>
                    </div>
                  </label>
                  
                  <label className="flex items-start gap-3">
                    <input
                      type="checkbox"
                      checked={mergeOptions.dropMissingColumns}
                      onChange={(e) => setMergeOptions({...mergeOptions, dropMissingColumns: e.target.checked})}
                      className="mt-1"
                    />
                    <div>
                      <p className="font-medium">Drop Missing Columns</p>
                      <p className="text-sm text-gray-600">Remove columns that don't exist in the new file</p>
                    </div>
                  </label>
                </div>
              </div>

              {/* Row Handling */}
              <div className="border-2 border-gray-200 rounded-lg p-4">
                <h4 className="font-semibold mb-3 flex items-center gap-2">
                  <Database className="w-4 h-4" />
                  Row Handling
                </h4>
                <div className="space-y-3">
                  <label className="flex items-start gap-3">
                    <input
                      type="checkbox"
                      checked={mergeOptions.updateExisting}
                      onChange={(e) => setMergeOptions({...mergeOptions, updateExisting: e.target.checked})}
                      className="mt-1"
                    />
                    <div>
                      <p className="font-medium">Update Existing Rows</p>
                      <p className="text-sm text-gray-600">Update rows that match based on key columns</p>
                    </div>
                  </label>
                  
                  <label className="flex items-start gap-3">
                    <input
                      type="checkbox"
                      checked={mergeOptions.insertNew}
                      onChange={(e) => setMergeOptions({...mergeOptions, insertNew: e.target.checked})}
                      className="mt-1"
                    />
                    <div>
                      <p className="font-medium">Insert New Rows</p>
                      <p className="text-sm text-gray-600">Add rows that don't exist in the current table</p>
                    </div>
                  </label>
                  
                  <label className="flex items-start gap-3">
                    <input
                      type="checkbox"
                      checked={mergeOptions.deleteMissing}
                      onChange={(e) => setMergeOptions({...mergeOptions, deleteMissing: e.target.checked})}
                      className="mt-1"
                    />
                    <div>
                      <p className="font-medium">Delete Missing Rows</p>
                      <p className="text-sm text-gray-600">Remove rows that don't exist in the new file</p>
                    </div>
                  </label>
                </div>
              </div>

              {/* Data Comparison */}
              <div className="border-2 border-gray-200 rounded-lg p-4">
                <h4 className="font-semibold mb-3 flex items-center gap-2">
                  <Settings className="w-4 h-4" />
                  Data Comparison
                </h4>
                <div className="space-y-3">
                  <label className="flex items-start gap-3">
                    <input
                      type="checkbox"
                      checked={mergeOptions.ignoreCase}
                      onChange={(e) => setMergeOptions({...mergeOptions, ignoreCase: e.target.checked})}
                      className="mt-1"
                    />
                    <div>
                      <p className="font-medium">Ignore Case</p>
                      <p className="text-sm text-gray-600">Treat 'Apple' and 'apple' as the same value</p>
                    </div>
                  </label>
                  
                  <label className="flex items-start gap-3">
                    <input
                      type="checkbox"
                      checked={mergeOptions.trimWhitespace}
                      onChange={(e) => setMergeOptions({...mergeOptions, trimWhitespace: e.target.checked})}
                      className="mt-1"
                    />
                    <div>
                      <p className="font-medium">Trim Whitespace</p>
                      <p className="text-sm text-gray-600">Remove leading/trailing spaces before comparison</p>
                    </div>
                  </label>
                  
                  <div>
                    <p className="font-medium mb-2">Handle Empty Values</p>
                    <select
                      value={mergeOptions.handleNulls}
                      onChange={(e) => setMergeOptions({...mergeOptions, handleNulls: e.target.value as 'keep' | 'empty' | 'null'})}
                      className="neo-input w-full"
                    >
                      <option value="keep">Keep existing values</option>
                      <option value="empty">Replace with empty string</option>
                      <option value="null">Replace with NULL</option>
                    </select>
                  </div>
                </div>
              </div>

              {/* Info Box */}
              <div className="p-4 bg-blue-50 border-2 border-blue-200 rounded-lg">
                <div className="flex items-start gap-2">
                  <Info className="w-5 h-5 text-blue-600 mt-0.5 flex-shrink-0" />
                  <div className="text-sm text-blue-800">
                    <p className="font-medium mb-1">Merge Behavior</p>
                    <p>The system will automatically detect matching columns and attempt to map renamed columns based on similarity. Rows will be matched using all columns as a composite key unless you specify key columns in your data.</p>
                  </div>
                </div>
              </div>
            </div>
            
            <div className="flex gap-3 mt-6">
              <button
                onClick={handleAdvancedMergeConfirm}
                className="flex-1 neo-button"
              >
                Apply Advanced Merge
              </button>
              <button
                onClick={() => {
                  setShowAdvancedOptions(false);
                  setShowConflictModal(true);
                }}
                className="flex-1 neo-button-secondary"
              >
                Back
              </button>
            </div>
          </div>
        </div>
      )}
    </>
  );
};

export default FileUploadTab;