import React, { useEffect, useState } from 'react';
import { AlertCircle, CheckCircle, ChevronDown, ChevronRight, Loader2, Search, X } from 'lucide-react';
import { TableInfo } from '../../../types/chat';

interface SchemaTabProps {
  isLoadingTables: boolean;
  tables: TableInfo[];
  selectedTables: string[];
  expandedTables: Record<string, boolean>;
  schemaSearchQuery: string;
  selectAllTables: boolean;
  schemaValidationError: string | null;
  setSchemaSearchQuery: (query: string) => void;
  toggleSelectAllTables: () => void;
  toggleExpandTable: (tableName: string, forceState?: boolean) => void;
  toggleTable: (tableName: string) => void;
}

const SchemaTab: React.FC<SchemaTabProps> = ({
  isLoadingTables,
  tables,
  selectedTables,
  expandedTables,
  schemaSearchQuery,
  selectAllTables,
  schemaValidationError,
  setSchemaSearchQuery,
  toggleSelectAllTables,
  toggleExpandTable,
  toggleTable,
}) => {
  

  // Filter tables based on search query
  const filteredTables = tables.filter(table => 
    table.name.toLowerCase().includes(schemaSearchQuery.toLowerCase())
  );

  return (
    <div className="space-y-6">
      {schemaValidationError && (
        <div className="p-4 bg-amber-50 border-2 border-amber-500 rounded-lg mb-4">
          <div className="flex items-center gap-2 text-amber-600">
            <AlertCircle className="w-5 h-5" />
            <p className="font-medium">{schemaValidationError}</p>
          </div>
        </div>
      )}

      {isLoadingTables ? (
        <div className="flex flex-col items-center justify-center py-12">
          <Loader2 className="w-10 h-10 animate-spin text-gray-400 mb-4" />
          <span className="text-gray-600 font-medium">Loading Knowledge Base Tables...</span>
          <p className="text-gray-500 text-sm mt-2 text-center max-w-md">
            This may take a moment depending on the size of your data.
            <br />
            Large data sources with many tables may take longer to load.
          </p>
        </div>
      ) : (
        <div className="space-y-4">
          <div className="neo-border p-4 rounded-lg">
            <label className="flex items-center gap-2 cursor-pointer">
              <input
                type="checkbox"
                className="w-5 h-5 rounded-md border-2 border-black"
                checked={selectAllTables}
                onChange={toggleSelectAllTables}
              />
              <span className="font-bold text-lg">All & Auto Detect</span>
            </label>
            <p className="text-gray-600 text-sm mt-1 ml-7">
              Select all tables in the database. This option auto detects the added/removed entities in the database.
            </p>
          </div>

          <div className="mt-4">
            <div className="flex flex-col md:flex-row gap-2 md:gap-0 justify-between items-start md:items-center mt-6 mb-2">
              <h3 className="font-bold text-lg">Knowledge Base Tables</h3>
              <div className="flex items-center gap-2">
                <span className="text-sm text-gray-600">
                  {selectAllTables ? 'All' : `${selectedTables?.length}/${tables?.length}`} selected
                </span>
                <button
                  type="button"
                  onClick={() => {
                    const allExpanded = Object.values(expandedTables).every(v => v);
                    const newExpandedState = !allExpanded;
                    
                    // Update all tables in one call by passing a special value
                    // The parent component will handle updating all tables
                    if (tables.length > 0) {
                      const tableNames = tables.map(table => table.name);
                      tableNames.forEach(name => toggleExpandTable(name, newExpandedState));
                    }
                  }}
                  className="text-xs bg-gray-100 hover:bg-gray-200 text-gray-700 px-2 py-1 rounded transition-colors"
                >
                  {Object.values(expandedTables).every(v => v) ? 'Collapse All' : 'Expand All'}
                </button>
              </div>
            </div>
            <p className="text-gray-600 text-sm mb-4">
              Or select specific tables to include in your knowledge base.
            </p>

            {/* Search input */}
            <div className="relative mb-4">
              <div className="absolute inset-y-0 left-0 pl-3 flex items-center pointer-events-none">
                <Search className="h-4 w-4 text-gray-400" />
              </div>
              <input
                type="text"
                placeholder="Search tables..."
                className="neo-input pl-8 w-full"
                value={schemaSearchQuery}
                onChange={(e) => setSchemaSearchQuery(e.target.value)}
              />
              {schemaSearchQuery && (
                <button
                  onClick={() => setSchemaSearchQuery('')}
                  className="absolute inset-y-0 right-0 pr-3 flex items-center"
                >
                  <X className="h-4 w-4 text-gray-400 hover:text-gray-600" />
                </button>
              )}
            </div>

            <div className="space-y-2 max-h-[40vh] overflow-y-auto neo-border p-4 rounded-lg">
              {filteredTables.length === 0 ? (
                <div className="text-center py-4 text-gray-500">
                  {schemaSearchQuery ? 'No tables match your search' : 'No tables found in database'}
                </div>
              ) : (
                filteredTables.map(table => (
                  <div key={table.name} className="border-b border-gray-200 last:border-b-0 pb-2">
                    <div className="flex items-center">
                      <button
                        onClick={() => toggleExpandTable(table.name)}
                        className="mr-2 p-1 hover:bg-gray-100 rounded"
                      >
                        {expandedTables[table.name] ? (
                          <ChevronDown className="w-4 h-4" />
                        ) : (
                          <ChevronRight className="w-4 h-4" />
                        )}
                      </button>
                      <label className="flex items-center gap-2 cursor-pointer flex-grow">
                        <input
                          type="checkbox"
                          className="w-4 h-4 rounded-md border-2 border-black checked:bg-green-500 checked:border-green-500 focus:ring-green-500 text-green-500"
                          checked={selectedTables.includes(table.name)}
                          onChange={() => toggleTable(table.name)}
                        />
                        <span className="font-medium">{table.name}</span>
                      </label>
                    </div>
                    
                    {expandedTables[table.name] && (
                      <div className="ml-8 mt-2 pl-2 border-l-2 border-gray-200">
                        <p className="text-sm font-medium text-gray-700 mb-2">Columns:</p>
                        <div className="space-y-1.5 max-h-[200px] overflow-y-auto pr-2">
                          {table.columns.map(column => (
                            <div key={column.name} className="flex items-start p-1.5 rounded-md hover:bg-gray-50">
                              <div className="flex-1">
                                <span className="font-medium text-sm inline-block">{column.name}</span>
                                <span className="text-xs bg-gray-100 text-gray-700 px-1.5 py-0.5 rounded ml-2">{column.type}</span>
                                {column.is_nullable && (
                                  <span className="text-xs bg-gray-100 text-gray-500 px-1.5 py-0.5 rounded ml-1">nullable</span>
                                )}
                              </div>
                            </div>
                          ))}
                        </div>
                      </div>
                    )}
                  </div>
                ))
              )}
            </div>
          </div>

          <div className="mt-6">
            <p className="text-gray-500 text-sm text-center">
              When tables selection is updated, knowledge base will be automatically refreshed in the background which may take 1-2 minutes depending on the size of your data.
            </p>
          </div>
        </div>
      )}
    </div>
  );
};

export default SchemaTab; 