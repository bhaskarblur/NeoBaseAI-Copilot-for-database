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
import Tooltip from '../../ui/Tooltip';
import toast from 'react-hot-toast';
import chatService from '../../../services/chatService';

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
  const [deleteConfirmation, setDeleteConfirmation] = useState<{ show: boolean; tableName: string | null; isSelectedRows?: boolean; rowId?: string | number }>({ show: false, tableName: null, isSelectedRows: false });
  const [downloadMenu, setDownloadMenu] = useState<{ show: boolean; tableName: string | null; x: number; y: number }>({ show: false, tableName: null, x: 0, y: 0 });
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (chatId) {
      loadTableData();
    }
  }, [chatId]);

  const loadTableData = async () => {
    setLoading(true);
    setError(null);
    try {
      const tablesResponse = await chatService.getTables(chatId);
      
      if (tablesResponse.tables && tablesResponse.tables.length > 0) {
        // Transform API response to match TableData structure
        const transformedTables = tablesResponse.tables.map((table: any) => ({
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
    } catch (error: any) {
      console.error('Failed to load table data:', error);
      setError(error.message || 'Failed to load table data');
      setTables([]);
      
      // Show error toast
      toast.error(error.message || 'Failed to load table data', {
        duration: 4000,
        style: {
          background: '#ff4444',
          color: '#fff',
          border: '4px solid #cc0000',
          borderRadius: '12px',
          boxShadow: '4px 4px 0px 0px rgba(0,0,0,1)',
          padding: '12px 24px',
          fontSize: '16px',
          fontWeight: 'bold',
        },
      });
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
        // Handle both wrapped and unwrapped responses
        const responseData = data.data || data;
        setTables(prev => prev.map(table => 
          table.name === tableName 
            ? { ...table, previewData: responseData.rows || [] }
            : table
        ));
        // Update pagination total if provided
        if (responseData.total_rows !== undefined) {
          setPagination(prev => ({
            ...prev,
            [tableName]: {
              ...prev[tableName],
              totalRows: responseData.total_rows
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
    navigator.clipboard.writeText(text).then(() => {
      toast.success('Row data copied to clipboard', {
        duration: 2000,
        style: {
          background: '#000',
          color: '#fff',
          border: '4px solid #000',
          borderRadius: '12px',
          boxShadow: '4px 4px 0px 0px rgba(0,0,0,1)',
          padding: '12px 24px',
          fontSize: '16px',
          fontWeight: '500',
        },
      });
    }).catch(() => {
      toast.error('Failed to copy row data', {
        duration: 2000,
        style: {
          background: '#ff4444',
          color: '#fff',
          border: '4px solid #cc0000',
          borderRadius: '12px',
          boxShadow: '4px 4px 0px 0px rgba(0,0,0,1)',
          padding: '12px 24px',
          fontSize: '16px',
          fontWeight: 'bold',
        },
      });
    });
  };

  const handleDownloadClick = (e: React.MouseEvent, tableName: string) => {
    e.stopPropagation();
    const rect = e.currentTarget.getBoundingClientRect();
    setDownloadMenu({ show: true, tableName, x: rect.left, y: rect.bottom + 5 });
  };

  const handleDownload = async (format: 'csv' | 'xlsx') => {
    if (downloadMenu.tableName) {
      const loadingToast = toast.loading('Preparing download...', {
        style: {
          background: '#000',
          color: '#fff',
          border: '4px solid #000',
          borderRadius: '12px',
          boxShadow: '4px 4px 0px 0px rgba(0,0,0,1)',
          padding: '12px 24px',
          fontSize: '16px',
          fontWeight: '500',
        },
      });

      try {
        // Check if we're downloading selected rows
        const selectedRowIndices = selectedRows[downloadMenu.tableName];
        const isSelectedDownload = selectedRowIndices && selectedRowIndices.size > 0;
        
        // Get the table data
        const table = tables.find(t => t.name === downloadMenu.tableName);
        if (!table) {
          throw new Error('Table not found');
        }

        // If we're downloading selected rows but no data is loaded, we can't proceed
        if (isSelectedDownload && (!table.previewData || table.previewData.length === 0)) {
          toast.dismiss(loadingToast);
          toast.error('Please expand the table first to load data before downloading selected rows', {
            duration: 3000,
            style: {
              background: '#ff4444',
              color: '#fff',
              border: '4px solid #cc0000',
              borderRadius: '12px',
              boxShadow: '4px 4px 0px 0px rgba(0,0,0,1)',
              padding: '12px 24px',
              fontSize: '16px',
              fontWeight: 'bold',
            },
          });
          return;
        }

        // For full table download, we'll use the server endpoint directly
        // For selected rows with CSV, we need the preview data
        if (format === 'csv' && isSelectedDownload) {
          const dataToDownload = table.previewData.filter((_, index) => selectedRowIndices.has(index));
          
          if (dataToDownload.length === 0) {
            toast.dismiss(loadingToast);
            toast.error('No data to download', {
              duration: 2000,
              style: {
                background: '#ff4444',
                color: '#fff',
                border: '4px solid #cc0000',
                borderRadius: '12px',
                boxShadow: '4px 4px 0px 0px rgba(0,0,0,1)',
                padding: '12px 24px',
                fontSize: '16px',
                fontWeight: 'bold',
              },
            });
            return;
          }

          // Create CSV content for selected rows
          const headers = table.columns.map(col => col.name).join(',');
          const rows = dataToDownload.map(row => 
            table.columns.map(col => {
              const value = row[col.name];
              if (value === null || value === undefined) return '';
              // Escape quotes and wrap in quotes if contains comma or newline
              const stringValue = String(value);
              if (stringValue.includes(',') || stringValue.includes('\n') || stringValue.includes('"')) {
                return `"${stringValue.replace(/"/g, '""')}"`;
              }
              return stringValue;
            }).join(',')
          );
          
          const csvContent = [headers, ...rows].join('\n');
          const blob = new Blob([csvContent], { type: 'text/csv;charset=utf-8;' });
          const url = window.URL.createObjectURL(blob);
          const a = document.createElement('a');
          a.href = url;
          a.download = `${downloadMenu.tableName}_selected.csv`;
          document.body.appendChild(a);
          a.click();
          window.URL.revokeObjectURL(url);
          document.body.removeChild(a);
        } else {
          // For full table downloads (CSV or XLSX) or selected XLSX, use server endpoint
          let url = `${import.meta.env.VITE_API_URL}/upload/${chatId}/tables/${downloadMenu.tableName}/download?format=${format}`;
          
          // Add selected row IDs to the URL if downloading selected rows
          if (isSelectedDownload && format === 'xlsx') {
            if (table.previewData && table.previewData.length > 0) {
              const selectedRowIds = table.previewData
                .filter((_, index) => selectedRowIndices.has(index))
                .map(row => row._id || '')
                .filter(id => id !== '');
              
              if (selectedRowIds.length > 0) {
                url += `&rowIds=${selectedRowIds.join(',')}`;
              }
            }
          }
          
          const response = await fetch(url, {
            method: 'GET',
            headers: {
              'Authorization': `Bearer ${localStorage.getItem('token')}`,
              'Accept': format === 'xlsx' 
                ? 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet'
                : 'text/csv'
            }
          });
          
          if (!response.ok) {
            const errorText = await response.text();
            console.error('Download failed:', errorText);
            throw new Error('Download failed');
          }

          const blob = await response.blob();
          const blobUrl = window.URL.createObjectURL(blob);
          const a = document.createElement('a');
          a.href = blobUrl;
          a.download = `${downloadMenu.tableName}${isSelectedDownload ? '_selected' : ''}.${format}`;
          document.body.appendChild(a);
          a.click();
          window.URL.revokeObjectURL(blobUrl);
          document.body.removeChild(a);
        }

        toast.dismiss(loadingToast);
        toast.success('Download complete', {
          duration: 2000,
          style: {
            background: '#000',
            color: '#fff',
            border: '4px solid #000',
            borderRadius: '12px',
            boxShadow: '4px 4px 0px 0px rgba(0,0,0,1)',
            padding: '12px 24px',
            fontSize: '16px',
            fontWeight: '500',
          },
        });
      } catch (error) {
        toast.dismiss(loadingToast);
        toast.error('Download failed', {
          duration: 2000,
          style: {
            background: '#ff4444',
            color: '#fff',
            border: '4px solid #cc0000',
            borderRadius: '12px',
            boxShadow: '4px 4px 0px 0px rgba(0,0,0,1)',
            padding: '12px 24px',
            fontSize: '16px',
            fontWeight: 'bold',
          },
        });
        console.error('Download failed:', error);
      }
    }
    setDownloadMenu({ show: false, tableName: null, x: 0, y: 0 });
  };

  const handleDeleteRow = async (tableName: string, rowId: number | string) => {
    const loadingToast = toast.loading('Deleting row...', {
      style: {
        background: '#000',
        color: '#fff',
        border: '4px solid #000',
        borderRadius: '12px',
        boxShadow: '4px 4px 0px 0px rgba(0,0,0,1)',
        padding: '12px 24px',
        fontSize: '16px',
        fontWeight: '500',
      },
    });

    try {
      const response = await fetch(
        `${import.meta.env.VITE_API_URL}/upload/${chatId}/tables/${tableName}/rows/${rowId}`,
        {
          method: 'DELETE',
          headers: {
            'Authorization': `Bearer ${localStorage.getItem('token')}`
          }
        }
      );

      if (response.ok) {
        toast.dismiss(loadingToast);
        toast.success('Row deleted successfully', {
          duration: 2000,
          style: {
            background: '#000',
            color: '#fff',
            border: '4px solid #000',
            borderRadius: '12px',
            boxShadow: '4px 4px 0px 0px rgba(0,0,0,1)',
            padding: '12px 24px',
            fontSize: '16px',
            fontWeight: '500',
          },
        });
        
        // Reload table data to update row count and size
        await loadTableData();
        
        // Reload the preview data
        const currentPagination = pagination[tableName] || { page: 1, pageSize: 15, totalRows: 0 };
        await loadTablePreviewData(tableName, currentPagination.page, currentPagination.pageSize);
      } else {
        throw new Error('Failed to delete row');
      }
    } catch (error) {
      toast.dismiss(loadingToast);
      toast.error('Failed to delete row', {
        duration: 2000,
        style: {
          background: '#ff4444',
          color: '#fff',
          border: '4px solid #cc0000',
          borderRadius: '12px',
          boxShadow: '4px 4px 0px 0px rgba(0,0,0,1)',
          padding: '12px 24px',
          fontSize: '16px',
          fontWeight: 'bold',
        },
      });
      console.error('Delete row failed:', error);
    }
  };

  const handleDeleteTable = async (tableName: string) => {
    // Dismiss the modal first
    setDeleteConfirmation({ show: false, tableName: null });
    
    try {
      if (deleteConfirmation.rowId !== undefined) {
        // Delete individual row
        await handleDeleteRow(tableName, deleteConfirmation.rowId);
      } else if (deleteConfirmation.isSelectedRows) {
        // Delete selected rows one by one
        const selectedRowIndices = selectedRows[tableName];
        const table = tables.find(t => t.name === tableName);
        if (!table || !selectedRowIndices) {
          throw new Error('Table or selected rows not found');
        }

        const loadingToast = toast.loading(`Deleting ${selectedRowIndices.size} rows...`, {
          style: {
            background: '#000',
            color: '#fff',
            border: '4px solid #000',
            borderRadius: '12px',
            boxShadow: '4px 4px 0px 0px rgba(0,0,0,1)',
            padding: '12px 24px',
            fontSize: '16px',
            fontWeight: '500',
          },
        });

        let successCount = 0;
        let errorCount = 0;

        // Get the row IDs to delete
        const rowsToDelete = Array.from(selectedRowIndices).map(index => {
          const row = table.previewData[index];
          return row?._id || index;
        });

        // Delete each row
        for (const rowId of rowsToDelete) {
          try {
            const response = await fetch(
              `${import.meta.env.VITE_API_URL}/upload/${chatId}/tables/${tableName}/rows/${rowId}`,
              {
                method: 'DELETE',
                headers: {
                  'Authorization': `Bearer ${localStorage.getItem('token')}`
                }
              }
            );
            
            if (response.ok) {
              successCount++;
            } else {
              errorCount++;
            }
          } catch (error) {
            errorCount++;
          }
        }

        toast.dismiss(loadingToast);
        
        if (errorCount === 0) {
          toast.success(`Successfully deleted ${successCount} rows`, {
            duration: 2000,
            style: {
              background: '#000',
              color: '#fff',
              border: '4px solid #000',
              borderRadius: '12px',
              boxShadow: '4px 4px 0px 0px rgba(0,0,0,1)',
              padding: '12px 24px',
              fontSize: '16px',
              fontWeight: '500',
            },
          });
        } else {
          toast.error(`Deleted ${successCount} rows, ${errorCount} failed`, {
            duration: 2000,
            style: {
              background: '#ff4444',
              color: '#fff',
              border: '4px solid #cc0000',
              borderRadius: '12px',
              boxShadow: '4px 4px 0px 0px rgba(0,0,0,1)',
              padding: '12px 24px',
              fontSize: '16px',
              fontWeight: 'bold',
            },
          });
        }

        // Clear selected rows after deletion
        setSelectedRows(prev => ({
          ...prev,
          [tableName]: new Set()
        }));

        // Reload the table data and preview
        await loadTableData();
        const currentPagination = pagination[tableName] || { page: 1, pageSize: 15, totalRows: 0 };
        await loadTablePreviewData(tableName, currentPagination.page, currentPagination.pageSize);
      } else {
        // Delete entire table
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
      }
    } catch (error) {
      console.error('Delete failed:', error);
    }
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
      <div className="flex flex-col items-center justify-center h-64 gap-4">
        <Loader2 className="w-8 h-8 animate-spin text-gray-500" />
        <div className="text-center text-gray-600 max-w-md">
          <p className="font-medium">Loading data structure...</p>
          <p className="text-sm mt-2">
            Fetching table schemas, column information, and data statistics from your database.
          </p>
        </div>
      </div>
    );
  }

  return (
    <div className="space-y-4">
      <div className="flex justify-between items-center mb-4">
        <h3 className="text-lg font-semibold">Data Structure</h3>
        {/* Commented for now, will be added back later */}
        {/* <Tooltip content="Refresh table data">
          <button
            onClick={onRefreshData}
            className="neo-button-secondary flex items-center gap-2 px-3 py-1.5 text-sm"
          >
            <RefreshCw className="w-4 h-4" />
            Refresh
          </button>
        </Tooltip> */}
      </div>

      {/* Error Display */}
      {error && (
        <div className="p-4 bg-red-50 rounded-lg border-2 border-red-200 mb-4">
          <div className="flex items-center gap-3">
            <AlertCircle className="w-5 h-5 text-red-600 flex-shrink-0" />
            <div>
              <h4 className="font-medium text-red-800">Failed to Load Data Structure</h4>
              <p className="text-sm text-red-700 mt-1">{error}</p>
              <button
                onClick={() => {
                  setError(null);
                  loadTableData();
                }}
                className="mt-2 text-sm text-red-600 hover:text-red-800 underline"
              >
                Try again
              </button>
            </div>
          </div>
        </div>
      )}

      {tables.length === 0 && !error ? (
        <div className="text-center py-8 text-gray-500">
          No tables found. Upload CSV/Excel files or sync Google Sheets to see data structure.
        </div>
      ) : tables.length > 0 ? (
        <div className="space-y-4">
          {tables.map((table) => (
            <div key={table.name} className="border-2 border-gray-200 rounded-lg overflow-hidden">
              {/* Table Header */}
              <div className="bg-gray-50 p-4 flex items-center justify-between">
                <div className="flex items-center gap-3">
                  <button
                    onClick={() => toggleTableExpansion(table.name)}
                    className="p-1 hover:bg-gray-200 rounded transition-colors"
                    title={expandedTables[table.name] ? "Collapse table" : "Expand table"}
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
                      Data: {table.rowCount.toLocaleString()} rows • Size: {formatBytes(table.sizeBytes)}
                    </div>
                  </div>
                </div>
                <div className="flex items-center gap-2">
                  <button
                    onClick={(e) => handleDownloadClick(e, table.name)}
                    className="p-2 hover:bg-gray-200 rounded transition-colors"
                    title="Download table data"
                  >
                    <Download className="w-4 w-4" />
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
                      <div className="flex items-center gap-2">
                        {selectedRows[table.name]?.size > 0 && (
                          <>
                            <span className="text-sm text-gray-500 mr-2">
                              ({selectedRows[table.name].size} selected)
                            </span>
                            <button
                              onClick={(e) => {
                                const rect = e.currentTarget.getBoundingClientRect();
                                setDownloadMenu({ show: true, tableName: table.name, x: rect.left, y: rect.bottom + 5 });
                              }}
                              className="p-1.5 hover:bg-gray-200 rounded transition-colors"
                              title="Download selected rows"
                            >
                              <Download className="w-4 h-4" />
                            </button>
                            <button
                              onClick={() => setDeleteConfirmation({ show: true, tableName: table.name, isSelectedRows: true })}
                              className="p-1.5 hover:bg-red-100 text-red-600 rounded transition-colors"
                              title="Delete selected rows"
                            >
                              <Trash2 className="w-4 h-4" />
                            </button>
                          </>
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
                              <th key={col.name} className="p-4 text-left font-medium text-sm bg-gray-100">
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
                                <td key={col.name} className="p-4 text-sm">
                                  <span className="truncate max-w-xs block" title={String(row[col.name] ?? '-')}>
                                    {row[col.name] === null || row[col.name] === undefined ? '-' : row[col.name]}
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
                                    <Copy className="w-3.5 h-3.5" />
                                  </button>
                                  <button
                                    onClick={() => setDeleteConfirmation({ show: true, tableName: table.name, rowId: row._id || rowIndex })}
                                    className="p-1 hover:bg-red-100 text-red-600 rounded"
                                    title="Delete row"
                                  >
                                    <Trash2 className="w-4 h-4" />
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
                            {((pagination[table.name]?.page || 1) - 1) * (pagination[table.name]?.pageSize || 15) + table.previewData.length} of {pagination[table.name]?.totalRows || table.rowCount}</>
                          ) : (
                            'No data'
                          )}
                        </span>
                        <div className="flex gap-1">
                          <button
                            onClick={() => handlePageChange(table.name, 1)}
                            disabled={(pagination[table.name]?.page || 1) === 1}
                            className="p-1 hover:bg-gray-200 rounded disabled:opacity-50 disabled:cursor-not-allowed"
                            title="First page"
                          >
                            <ChevronsLeft className="w-4 h-4" />
                          </button>
                          <button
                            onClick={() => handlePageChange(table.name, (pagination[table.name]?.page || 1) - 1)}
                            disabled={(pagination[table.name]?.page || 1) === 1}
                            className="p-1 hover:bg-gray-200 rounded disabled:opacity-50 disabled:cursor-not-allowed"
                            title="Previous page"
                          >
                            <ChevronLeft className="w-4 h-4" />
                          </button>
                          <button
                            onClick={() => handlePageChange(table.name, (pagination[table.name]?.page || 1) + 1)}
                            disabled={(pagination[table.name]?.page || 1) * (pagination[table.name]?.pageSize || 15) >= (pagination[table.name]?.totalRows || table.rowCount)}
                            className="p-1 hover:bg-gray-200 rounded disabled:opacity-50 disabled:cursor-not-allowed"
                            title="Next page"
                          >
                            <ChevronRightIcon className="w-4 h-4" />
                          </button>
                          <button
                            onClick={() => handlePageChange(table.name, Math.ceil((pagination[table.name]?.totalRows || table.rowCount) / (pagination[table.name]?.pageSize || 15)))}
                            disabled={(pagination[table.name]?.page || 1) * (pagination[table.name]?.pageSize || 15) >= (pagination[table.name]?.totalRows || table.rowCount)}
                            className="p-1 hover:bg-gray-200 rounded disabled:opacity-50 disabled:cursor-not-allowed"
                            title="Last page"
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
      ) : null}

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
          title={
            deleteConfirmation.rowId !== undefined 
              ? "Delete Row" 
              : deleteConfirmation.isSelectedRows 
                ? "Delete Selected Rows" 
                : "Delete Table"
          }
          message={
            deleteConfirmation.rowId !== undefined
              ? `Are you sure you want to delete this row from "${deleteConfirmation.tableName}"? This action cannot be undone.`
              : deleteConfirmation.isSelectedRows
                ? `Are you sure you want to delete ${selectedRows[deleteConfirmation.tableName]?.size || 0} selected rows from "${deleteConfirmation.tableName}"? This action cannot be undone.`
                : `Are you sure you want to delete the table "${deleteConfirmation.tableName}"? This action cannot be undone and all data in this table will be permanently deleted.`
          }
          onConfirm={() => handleDeleteTable(deleteConfirmation.tableName!)}
          onCancel={() => setDeleteConfirmation({ show: false, tableName: null, isSelectedRows: false, rowId: undefined })}
        />
      )}
    </div>
  );
}