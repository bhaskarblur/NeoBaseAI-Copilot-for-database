import React, { useState, useEffect } from 'react';
import { 
  ChevronDown, 
  ChevronRight, 
  Loader2, 
  Trash2, 
  Download, 
  Copy, 
  RefreshCw,
  ChevronLeft,
  ChevronRight as ChevronRightIcon,
  ChevronsLeft,
  ChevronsRight,
  AlertCircle
} from 'lucide-react';
import ConfirmationModal from '../../modals/ConfirmationModal';

interface DataStructureTabProps {
  chatId: string;
  isLoadingData?: boolean;
  onDeleteTable?: (tableName: string) => void;
  onDownloadData?: (tableName: string) => void;
  onRefreshData?: () => void;
}

interface TableData {
  name: string;
  columns: Array<{
    name: string;
    type: string;
  }>;
  rowCount: number;
  sizeBytes: number;
  sourceFile: string;
  previewData: any[];
}

interface PaginationState {
  page: number;
  pageSize: number;
  totalRows: number;
}

export default function DataStructureTab({
  chatId,
  isLoadingData = false,
  onDeleteTable,
  onDownloadData,
  onRefreshData
}: DataStructureTabProps) {
  const [tables, setTables] = useState<TableData[]>([]);
  const [expandedTables, setExpandedTables] = useState<Record<string, boolean>>({});
  const [selectedRows, setSelectedRows] = useState<Record<string, Set<number>>>({});
  const [pagination, setPagination] = useState<Record<string, PaginationState>>({});
  const [loading, setLoading] = useState(false);
  const [loadingPreview, setLoadingPreview] = useState<Record<string, boolean>>({});
  const [deleteConfirmation, setDeleteConfirmation] = useState<{ show: boolean; tableName: string | null }>({ show: false, tableName: null });
  const [downloadMenu, setDownloadMenu] = useState<{ show: boolean; tableName: string | null; x: number; y: number }>({ show: false, tableName: null, x: 0, y: 0 });

  useEffect(() => {
    if (chatId) {
      loadTableData();
    }
  }, [chatId]);

  const loadTableData = async () => {
    setLoading(true);
    try {
      const response = await fetch(`${import.meta.env.VITE_API_URL}/chats/${chatId}/tables`, {
        headers: {
          'Authorization': `Bearer ${localStorage.getItem('token')}`
        }
      });
      if (response.ok) {
        const data = await response.json();
        if (data.data?.tables && data.data.tables.length > 0) {
          // Transform API response to match TableData structure
          const transformedTables = data.data.tables.map((table: any) => ({
            name: table.name,
            columns: table.columns || [],
            rowCount: table.row_count || 0,
            sizeBytes: table.size_bytes || 0,
            sourceFile: table.source_file || 'uploaded file',
            previewData: []
          }));
          setTables(transformedTables);
          
          // Initialize pagination for each table
          const initialPagination: Record<string, PaginationState> = {};
          transformedTables.forEach((table: any) => {
            initialPagination[table.name] = {
              page: 1,
              pageSize: 15,
              totalRows: table.rowCount || 0
            };
          });
          setPagination(initialPagination);
          // Load preview data for first table by default
          if (transformedTables.length > 0) {
            setExpandedTables({ [transformedTables[0].name]: true });
            loadTablePreviewData(transformedTables[0].name, 1, 15);
          }
        } else {
          setTables([]);
        }
      }
    } catch (error) {
      console.error('Failed to load table data:', error);
      setTables([]);
    } finally {
      setLoading(false);
    }
  };

  const loadTablePreviewData = async (tableName: string, page: number = 1, pageSize: number = 15) => {
    setLoadingPreview(prev => ({ ...prev, [tableName]: true }));
    try {
      const response = await fetch(
        `${import.meta.env.VITE_API_URL}/upload/${chatId}/tables/${tableName}?page=${page}&pageSize=${pageSize}`,
        {
          headers: {
            'Authorization': `Bearer ${localStorage.getItem('token')}`
          }
        }
      );
      if (response.ok) {
        const data = await response.json();
        setTables(prev => prev.map(table => 
          table.name === tableName 
            ? { ...table, previewData: data.data?.rows || [] }
            : table
        ));
        // Update pagination total if provided
        if (data.data?.total !== undefined) {
          setPagination(prev => ({
            ...prev,
            [tableName]: {
              ...prev[tableName],
              totalRows: data.data.total
            }
          }));
        }
      }
    } catch (error) {
      console.error('Failed to load preview data:', error);
    } finally {
      setLoadingPreview(prev => ({ ...prev, [tableName]: false }));
    }
  };

  const toggleTableExpansion = async (tableName: string) => {
    const isExpanding = !expandedTables[tableName];
    setExpandedTables(prev => ({
      ...prev,
      [tableName]: isExpanding
    }));
    
    // Load preview data when expanding
    if (isExpanding) {
      const table = tables.find(t => t.name === tableName);
      if (table && table.previewData.length === 0) {
        const currentPagination = pagination[tableName] || { page: 1, pageSize: 15, totalRows: 0 };
        await loadTablePreviewData(tableName, currentPagination.page, currentPagination.pageSize);
      }
    }
  };

  const handleRowSelection = (tableName: string, rowIndex: number) => {
    setSelectedRows(prev => {
      const tableSelections = new Set(prev[tableName] || []);
      if (tableSelections.has(rowIndex)) {
        tableSelections.delete(rowIndex);
      } else {
        tableSelections.add(rowIndex);
      }
      return {
        ...prev,
        [tableName]: tableSelections
      };
    });
  };

  const handleSelectAllRows = (tableName: string, data: any[]) => {
    setSelectedRows(prev => {
      const isAllSelected = prev[tableName]?.size === data.length;
      if (isAllSelected) {
        return {
          ...prev,
          [tableName]: new Set()
        };
      } else {
        return {
          ...prev,
          [tableName]: new Set(data.map((_, index) => index))
        };
      }
    });
  };

  const handleCopyData = (data: any) => {
    const text = typeof data === 'object' ? JSON.stringify(data) : String(data);
    navigator.clipboard.writeText(text);
  };

  const handleCopyRow = (row: any) => {
    const text = JSON.stringify(row, null, 2);
    navigator.clipboard.writeText(text);
  };

  const handleDownloadClick = (e: React.MouseEvent, tableName: string) => {
    e.stopPropagation();
    const rect = e.currentTarget.getBoundingClientRect();
    setDownloadMenu({ show: true, tableName, x: rect.left, y: rect.bottom + 5 });
  };

  const handleDownload = async (format: 'csv' | 'xlsx') => {
    if (downloadMenu.tableName) {
      try {
        const response = await fetch(
          `${import.meta.env.VITE_API_URL}/upload/${chatId}/tables/${downloadMenu.tableName}/download?format=${format}`,
          {
            headers: {
              'Authorization': `Bearer ${localStorage.getItem('token')}`
            }
          }
        );
        if (response.ok) {
          const blob = await response.blob();
          const url = window.URL.createObjectURL(blob);
          const a = document.createElement('a');
          a.href = url;
          a.download = `${downloadMenu.tableName}.${format}`;
          document.body.appendChild(a);
          a.click();
          window.URL.revokeObjectURL(url);
          document.body.removeChild(a);
        }
      } catch (error) {
        console.error('Download failed:', error);
      }
    }
    setDownloadMenu({ show: false, tableName: null, x: 0, y: 0 });
  };

  const handleDeleteTable = async (tableName: string) => {
    try {
      const response = await fetch(
        `${import.meta.env.VITE_API_URL}/upload/${chatId}/tables/${tableName}`,
        {
          method: 'DELETE',
          headers: {
            'Authorization': `Bearer ${localStorage.getItem('token')}`
          }
        }
      );
      if (response.ok) {
        setTables(prev => prev.filter(t => t.name !== tableName));
        onDeleteTable?.(tableName);
      }
    } catch (error) {
      console.error('Delete failed:', error);
    }
    setDeleteConfirmation({ show: false, tableName: null });
  };

  const formatBytes = (bytes: number): string => {
    if (bytes === 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
  };

  const handlePageChange = async (tableName: string, newPage: number) => {
    const currentPagination = pagination[tableName] || { page: 1, pageSize: 15, totalRows: 0 };
    setPagination(prev => ({
      ...prev,
      [tableName]: {
        ...prev[tableName],
        page: newPage
      }
    }));
    // Load new page data
    await loadTablePreviewData(tableName, newPage, currentPagination.pageSize);
  };

  const handlePageSizeChange = async (tableName: string, newSize: number) => {
    setPagination(prev => ({
      ...prev,
      [tableName]: {
        ...prev[tableName],
        pageSize: newSize,
        page: 1 // Reset to first page
      }
    }));
    // Reload data with new page size
    await loadTablePreviewData(tableName, 1, newSize);
  };

  if (loading || isLoadingData) {
    return (
      <div className="flex items-center justify-center h-64">
        <Loader2 className="w-8 h-8 animate-spin text-gray-500" />
      </div>
    );
  }

  return (
    <div className="space-y-4">
      <div className="flex justify-between items-center mb-4">
        <h3 className="text-lg font-semibold">Data Structure</h3>
        <button
          onClick={onRefreshData}
          className="neo-button-secondary flex items-center gap-2 px-3 py-1.5 text-sm"
        >
          <RefreshCw className="w-4 h-4" />
          Refresh
        </button>
      </div>

      {tables.length === 0 ? (
        <div className="text-center py-8 text-gray-500">
          No tables found. Upload CSV or Excel files to see data structure.
        </div>
      ) : (
        <div className="space-y-4">
          {tables.map((table) => (
            <div key={table.name} className="border-2 border-gray-200 rounded-lg overflow-hidden">
              {/* Table Header */}
              <div className="bg-gray-50 p-4 flex items-center justify-between">
                <div className="flex items-center gap-3">
                  <button
                    onClick={() => toggleTableExpansion(table.name)}
                    className="p-1 hover:bg-gray-200 rounded transition-colors"
                  >
                    {expandedTables[table.name] ? (
                      <ChevronDown className="w-5 h-5" />
                    ) : (
                      <ChevronRight className="w-5 h-5" />
                    )}
                  </button>
                  <div>
                    <h4 className="font-semibold text-lg">{table.name}</h4>
                    <div className="text-sm text-gray-600">
                      Data: {table.rowCount.toLocaleString()} rows • Size: {formatBytes(table.sizeBytes)} • Source: {table.sourceFile}
                    </div>
                  </div>
                </div>
                <div className="flex items-center gap-2">
                  <button
                    onClick={(e) => handleDownloadClick(e, table.name)}
                    className="p-2 hover:bg-gray-200 rounded transition-colors"
                    title="Download table data"
                  >
                    <Download className="w-4 h-4" />
                  </button>
                  <button
                    onClick={() => setDeleteConfirmation({ show: true, tableName: table.name })}
                    className="p-2 hover:bg-red-100 text-red-600 rounded transition-colors"
                    title="Delete table"
                  >
                    <Trash2 className="w-4 h-4" />
                  </button>
                </div>
              </div>

              {/* Expanded Content */}
              {expandedTables[table.name] && (
                <div className="border-t-2 border-gray-200">
                  {/* Data Preview */}
                  <div className="p-4">
                    <div className="flex justify-between items-center mb-3">
                      <h5 className="font-medium">Data Preview</h5>
                      <div className="flex items-center gap-2 text-sm">
                        <button
                          onClick={() => handleSelectAllRows(table.name, table.previewData)}
                          className="text-blue-600 hover:text-blue-700"
                        >
                          {selectedRows[table.name]?.size === table.previewData.length ? 'Deselect All' : 'Select All'}
                        </button>
                        {selectedRows[table.name]?.size > 0 && (
                          <span className="text-gray-500">
                            ({selectedRows[table.name].size} selected)
                          </span>
                        )}
                      </div>
                    </div>

                    <div className="overflow-x-auto relative">
                      {loadingPreview[table.name] && (
                        <div className="absolute inset-0 bg-white bg-opacity-75 flex items-center justify-center z-10">
                          <Loader2 className="w-6 h-6 animate-spin text-gray-500" />
                        </div>
                      )}
                      <table className="w-full border-collapse">
                        <thead>
                          <tr className="border-b-2 border-gray-200 bg-gray-100">
                            <th className="p-2 text-left bg-gray-100">
                              <input
                                type="checkbox"
                                checked={selectedRows[table.name]?.size === table.previewData.length}
                                onChange={() => handleSelectAllRows(table.name, table.previewData)}
                                className="rounded border-gray-300"
                              />
                            </th>
                            {table.columns.map((col) => (
                              <th key={col.name} className="p-2 text-left font-medium text-sm bg-gray-100">
                                {col.name}
                              </th>
                            ))}
                            <th className="p-2 text-left bg-gray-100">Actions</th>
                          </tr>
                        </thead>
                        <tbody>
                          {table.previewData.length > 0 ? table.previewData.map((row, rowIndex) => (
                            <tr key={rowIndex} className="border-b hover:bg-gray-50 group">
                              <td className="p-2">
                                <input
                                  type="checkbox"
                                  checked={selectedRows[table.name]?.has(rowIndex) || false}
                                  onChange={() => handleRowSelection(table.name, rowIndex)}
                                  className="rounded border-gray-300"
                                />
                              </td>
                              {table.columns.map((col) => (
                                <td key={col.name} className="p-2 text-sm">
                                  <span className="truncate max-w-xs block" title={String(row[col.name])}>
                                    {row[col.name]}
                                  </span>
                                </td>
                              ))}
                              <td className="p-2">
                                <div className="flex items-center gap-1 opacity-0 group-hover:opacity-100">
                                  <button
                                    onClick={() => handleCopyRow(row)}
                                    className="p-1 hover:bg-gray-200 rounded"
                                    title="Copy row data"
                                  >
                                    <Copy className="w-3 h-3" />
                                  </button>
                                  <button
                                    onClick={() => {/* TODO: Delete row */}}
                                    className="p-1 hover:bg-red-100 text-red-600 rounded"
                                    title="Delete row"
                                  >
                                    <Trash2 className="w-3 h-3" />
                                  </button>
                                </div>
                              </td>
                            </tr>
                          )) : (
                            <tr>
                              <td colSpan={table.columns.length + 2} className="text-center py-8 text-gray-500">
                                No data to display
                              </td>
                            </tr>
                          )}
                        </tbody>
                      </table>
                    </div>

                    {/* Pagination Controls */}
                    <div className="flex items-center justify-between mt-4">
                      <div className="flex items-center gap-2">
                        <span className="text-sm text-gray-600">Rows per page:</span>
                        <select
                          value={pagination[table.name]?.pageSize || 15}
                          onChange={(e) => handlePageSizeChange(table.name, Number(e.target.value))}
                          className="neo-input px-2 py-1 text-sm"
                        >
                          <option value={10}>10</option>
                          <option value={15}>15</option>
                          <option value={25}>25</option>
                          <option value={50}>50</option>
                          <option value={100}>100</option>
                        </select>
                      </div>

                      <div className="flex items-center gap-2">
                        <span className="text-sm text-gray-600">
                          {table.previewData.length > 0 ? (
                            <>Showing {((pagination[table.name]?.page || 1) - 1) * (pagination[table.name]?.pageSize || 15) + 1}-
                            {Math.min((pagination[table.name]?.page || 1) * (pagination[table.name]?.pageSize || 15), pagination[table.name]?.totalRows || table.rowCount)} of {pagination[table.name]?.totalRows || table.rowCount}</>
                          ) : (
                            'No data'
                          )}
                        </span>
                        <div className="flex gap-1">
                          <button
                            onClick={() => handlePageChange(table.name, 1)}
                            disabled={(pagination[table.name]?.page || 1) === 1}
                            className="p-1 hover:bg-gray-200 rounded disabled:opacity-50 disabled:cursor-not-allowed"
                          >
                            <ChevronsLeft className="w-4 h-4" />
                          </button>
                          <button
                            onClick={() => handlePageChange(table.name, (pagination[table.name]?.page || 1) - 1)}
                            disabled={(pagination[table.name]?.page || 1) === 1}
                            className="p-1 hover:bg-gray-200 rounded disabled:opacity-50 disabled:cursor-not-allowed"
                          >
                            <ChevronLeft className="w-4 h-4" />
                          </button>
                          <button
                            onClick={() => handlePageChange(table.name, (pagination[table.name]?.page || 1) + 1)}
                            disabled={(pagination[table.name]?.page || 1) * (pagination[table.name]?.pageSize || 15) >= (pagination[table.name]?.totalRows || table.rowCount)}
                            className="p-1 hover:bg-gray-200 rounded disabled:opacity-50 disabled:cursor-not-allowed"
                          >
                            <ChevronRightIcon className="w-4 h-4" />
                          </button>
                          <button
                            onClick={() => handlePageChange(table.name, Math.ceil((pagination[table.name]?.totalRows || table.rowCount) / (pagination[table.name]?.pageSize || 15)))}
                            disabled={(pagination[table.name]?.page || 1) * (pagination[table.name]?.pageSize || 15) >= (pagination[table.name]?.totalRows || table.rowCount)}
                            className="p-1 hover:bg-gray-200 rounded disabled:opacity-50 disabled:cursor-not-allowed"
                          >
                            <ChevronsRight className="w-4 h-4" />
                          </button>
                        </div>
                      </div>
                    </div>
                  </div>
                </div>
              )}
            </div>
          ))}
        </div>
      )}

      {/* Download Menu Popup */}
      {downloadMenu.show && (
        <>
          <div
            className="fixed inset-0 z-10"
            onClick={() => setDownloadMenu({ show: false, tableName: null, x: 0, y: 0 })}
          />
          <div
            className="fixed z-20 bg-white border-2 border-gray-200 rounded-lg shadow-lg py-1"
            style={{ left: downloadMenu.x, top: downloadMenu.y }}
          >
            <button
              onClick={() => handleDownload('csv')}
              className="w-full px-4 py-2 text-left hover:bg-gray-100 text-sm"
            >
              Download as CSV
            </button>
            <button
              onClick={() => handleDownload('xlsx')}
              className="w-full px-4 py-2 text-left hover:bg-gray-100 text-sm"
            >
              Download as XLSX
            </button>
          </div>
        </>
      )}

      {/* Delete Confirmation Modal */}
      {deleteConfirmation.show && deleteConfirmation.tableName && (
        <ConfirmationModal
          title="Delete Table"
          message={`Are you sure you want to delete the table "${deleteConfirmation.tableName}"? This action cannot be undone and all data in this table will be permanently deleted.`}
          onConfirm={() => handleDeleteTable(deleteConfirmation.tableName!)}
          onCancel={() => setDeleteConfirmation({ show: false, tableName: null })}
        />
      )}
    </div>
  );
}