import { AlertCircle, CheckCircle, ChevronDown, Database, Globe, KeyRound, Loader2, RefreshCcw, Settings, Table, X } from 'lucide-react';
import { BasicConnectionTab, SchemaTab, SettingsTab, SSHConnectionTab, FileUploadTab, DataStructureTab, GoogleSheetsTab } from './components';
import ConfirmationModal from './ConfirmationModal';
import { useConnectionModal, ConnectionModalProps, ModalTab } from '../../hooks/useConnectionModal';
import { validateField } from '../../utils/connectionValidation';

export type { ModalTab };

export default function ConnectionModal(props: ConnectionModalProps) {
  const {
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
    mongoUriInputRef,
    mongoUriSshInputRef,
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
  } = useConnectionModal(props);

  const { initialData, onClose } = props;
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
                    { value: 'google_sheets', label: 'Google Sheets' },
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
            
            {/* Connection type tabs - Hide for CSV/Excel and Google Sheets */}
            {formData.type !== 'spreadsheet' && formData.type !== 'google_sheets' && (
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
                    <Globe className="w-4 h-4" />
                    <span>Basic</span>
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
                onFilesChange={handleFilesChange}
                isEditMode={!!initialData || (showingNewlyCreatedSchema && !!newChatId)}
                chatId={initialData?.id || newChatId}
                preloadedTables={tables}
              />
            ) : formData.type === 'google_sheets' ? (
              <GoogleSheetsTab
                formData={formData}
                handleChange={handleChange}
                isEditMode={!!initialData}
                onRefreshData={() => setShowRefreshSchema(true)}
                onGoogleAuthChange={handleGoogleAuthChange}
              />
            ) : connectionType === 'basic' ? (
              <BasicConnectionTab
                formData={formData}
                errors={errors}
                touched={touched}
                handleChange={handleChange}
                handleBlur={handleBlur}
                validateField={validateField}
                mongoUriInputRef={mongoUriInputRef}
                onMongoUriChange={setMongoUriValue}
              />
            ) : (
              <SSHConnectionTab
                formData={formData}
                errors={errors}
                touched={touched}
                handleChange={handleChange}
                handleBlur={handleBlur}
                validateField={validateField}
                mongoUriSshInputRef={mongoUriSshInputRef}
                onMongoUriChange={setMongoUriSshValue}
              />
            )}
          </>
        );
      case 'schema':
        return formData.type === 'spreadsheet' || formData.type === 'google_sheets' ? (
          <DataStructureTab
            chatId={newChatId || initialData?.id || ''}
            isLoadingData={isLoadingTables}
            onDeleteTable={(tableName) => {
              console.log('DataStructureTab: Delete table requested:', tableName);
            }}
            onDownloadData={(tableName) => {
              console.log('DataStructureTab: Download data requested:', tableName);
            }}
            onRefreshData={reloadTables}
          />
        ) : (
          <SchemaTab
            chatId={newChatId || initialData?.id || ''}
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
            autoGenerateVisualization={autoGenerateVisualization}
            setAutoExecuteQuery={setAutoExecuteQuery}
            setShareWithAI={setShareWithAI}
            setNonTechMode={setNonTechMode}
            setAutoGenerateVisualization={setAutoGenerateVisualization}
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
                  {initialData ? 'Chat Settings' : 'New Connection'}
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
        <div className="flex border-b border-gray-200 px-2 flex-shrink-0 overflow-x-auto">
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
              <span className="">Connection</span>
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
                <span className="flex flex-row">Knowledge <span className='hidden md:flex ml-1'>Base</span></span>
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
              <span className="">Settings</span>
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
            {initialData && !successMessage && !isLoading && activeTab === 'connection' && formData.type !== 'spreadsheet' && formData.type !== 'google_sheets' && (
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
                      ? ((formData.type === 'spreadsheet' || formData.type === 'google_sheets') ? 'Save Knowledge' : 'Save Knowledge') 
                      : (showingNewlyCreatedSchema && activeTab === 'connection' && formData.type === 'spreadsheet')
                        ? 'Upload Files'
                        : 'Create' 
                    : activeTab === 'settings' 
                      ? 'Update Settings' 
                      : activeTab === 'schema' 
                        ? ((formData.type === 'spreadsheet' || formData.type === 'google_sheets') ? 'Update Knowledge' : 'Update Knowledge') 
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
    
    {/* Refresh Schema Modal */}
    {showRefreshSchema && (
      <ConfirmationModal
        icon={<RefreshCcw className="w-6 h-6 text-black" />}
        themeColor="black"
        title="Refresh Knowledge Base"
        buttonText="Refresh"
        message="This action will refetch the data from Google Sheets and update the knowledge base. This may take a few minutes depending on the size of the sheet."
        onConfirm={handleRefreshSchema}
        onCancel={() => setShowRefreshSchema(false)}
        zIndex="z-[210]"
      />
    )}
  </div>
);
}