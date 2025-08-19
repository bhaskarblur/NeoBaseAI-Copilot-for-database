import { EventSourcePolyfill } from 'event-source-polyfill';
import {
  ArrowRight,
  Boxes,
  Clock,
  Copy,
  HelpCircle,
  Link,
  Loader2,
  MoreVertical,
  PanelLeft,
  PanelLeftClose,
  Pencil,
  Plus,
  Trash2,
  X
} from 'lucide-react';
import { useCallback, useEffect, useRef, useState } from 'react';
import { toast } from 'react-hot-toast';
import { useNavigate, useLocation } from 'react-router-dom';
import { useUser } from '../../contexts/UserContext';
import chatService from '../../services/chatService';
import analyticsService, { trackViewGuideTutorial } from '../../services/analyticsService';
import { Chat } from '../../types/chat';
import DatabaseLogo from '../icons/DatabaseLogos';
import ConfirmationModal from '../modals/ConfirmationModal';
import DeleteConnectionModal from '../modals/DeleteConnectionModal';
import DuplicateChatModal from '../modals/DuplicateChatModal';
import { DemoModal } from '../modals/DemoModal';

export interface Connection {
  id: string;
  name: string;
  type: 'postgresql' | 'yugabytedb' | 'mysql' | 'clickhouse' | 'mongodb' | 'redis' | 'neo4j' | 'spreadsheet';
}

interface SidebarProps {
  isExpanded: boolean;
  onToggleExpand: () => void;
  connections: Chat[];
  onSelectConnection: (id: string) => void;
  onAddConnection: () => void;
  onLogout: () => void;
  onDeleteConnection?: (id: string) => void;
  onEditConnection?: () => void;
  onDuplicateConnection?: (id: string) => void;
  selectedConnection?: Chat;
  isLoadingConnections: boolean;
  onConnectionStatusChange?: (chatId: string, isConnected: boolean, from: string) => void;
  eventSource: EventSourcePolyfill | null;
}

// Date formatting and grouping functions
const formatDate = (dateString: string) => {
  const date = new Date(dateString);
  return date.toLocaleDateString('en-US', {
    month: 'short',
    day: 'numeric',
    year: 'numeric'
  });
};

// Function to get a user-friendly relative time (e.g., "2 hours ago")
const getRelativeTime = (dateString: string): string => {
  const date = new Date(dateString);
  const now = new Date();
  const diffTime = now.getTime() - date.getTime();
  const diffMinutes = Math.floor(diffTime / (1000 * 60));
  
  if (diffMinutes < 1) {
    return 'Just now';
  } else if (diffMinutes < 60) {
    return `${diffMinutes} minute${diffMinutes > 1 ? 's' : ''} ago`;
  } else {
    const diffHours = Math.floor(diffMinutes / 60);
    if (diffHours < 24) {
      return `${diffHours} hour${diffHours > 1 ? 's' : ''} ago`;
    } else {
      const diffDays = Math.floor(diffHours / 24);
      if (diffDays === 1) {
        return 'Yesterday';
      } else {
        return formatDate(dateString);
      }
    }
  }
};

// Function to determine the date group for a connection
const getDateGroup = (dateString: string): string => {
  const date = new Date(dateString);
  const now = new Date();
  const diffTime = now.getTime() - date.getTime();
  const diffDays = Math.floor(diffTime / (1000 * 60 * 60 * 24));
  
  if (diffDays === 0) {
    return 'Today';
  } else if (diffDays < 7) {
    return '7 Days';
  } else if (diffDays < 30) {
    return '30 Days';
  } else {
    return date.toLocaleDateString('en-US', { year: 'numeric', month: 'long' });
  }
};

// Group connections by date categories
const groupConnectionsByDate = (connections: Chat[]): Map<string, Chat[]> => {
  const groups = new Map<string, Chat[]>();
  
  // Sort connections by updated_at date (newest first)
  const sortedConnections = [...connections].sort(
    (a, b) => new Date(b.updated_at).getTime() - new Date(a.updated_at).getTime()
  );
  
  // Group connections by date
  sortedConnections.forEach(connection => {
    const group = getDateGroup(connection.updated_at);
    if (!groups.has(group)) {
      groups.set(group, []);
    }
    groups.get(group)?.push(connection);
  });
  
  return groups;
};

// Sort date groups in chronological order
const getSortedGroups = (groups: Map<string, Chat[]>): Array<[string, Chat[]]> => {
  const specialOrder: { [key: string]: number } = {
    'Today': 0,
    '7 Days': 1,
    '30 Days': 2
  };
  
  return Array.from(groups.entries()).sort((a, b) => {
    // Handle special groups first
    if (a[0] in specialOrder && b[0] in specialOrder) {
      return specialOrder[a[0]] - specialOrder[b[0]];
    } else if (a[0] in specialOrder) {
      return -1;
    } else if (b[0] in specialOrder) {
      return 1;
    }
    
    // Sort other groups (months) by date, newest first
    const dateA = new Date(a[0]);
    const dateB = new Date(b[0]);
    return dateB.getTime() - dateA.getTime();
  });
};

export default function Sidebar({
  isExpanded,
  onToggleExpand,
  connections,
  onSelectConnection,
  onAddConnection,
  onLogout,
  onDeleteConnection,
  onEditConnection,
  onDuplicateConnection,
  selectedConnection,
  isLoadingConnections,
  onConnectionStatusChange,
}: SidebarProps) {
  const [showLogoutConfirm, setShowLogoutConfirm] = useState(false);
  const [connectionToDelete, setConnectionToDelete] = useState<Chat | null>(null);
  const [connectionToDuplicate, setConnectionToDuplicate] = useState<Chat | null>(null);
  const [currentConnectedChatId, setCurrentConnectedChatId] = useState<string | null>(null);
  const previousConnectionRef = useRef<string | null>(null);
  const { user } = useUser();
  const [openConnectionMenu, setOpenConnectionMenu] = useState<string | null>(null);
  const [menuPosition, setMenuPosition] = useState<{ top: number; left: number } | null>(null);
  const [showDemoModal, setShowDemoModal] = useState(false);
  const [isTutorialClosed, setIsTutorialClosed] = useState(false);
  const navigate = useNavigate();
  const location = useLocation();
  
  useEffect(() => {
    const handleClickOutside = (event: MouseEvent) => {
      if (openConnectionMenu) {
        const target = event.target as HTMLElement;
        if (!target.closest('.connection-menu-container') && !target.closest('.connection-dropdown-menu')) {
          setOpenConnectionMenu(null);
          setMenuPosition(null);
        }
      }
    };
    
    document.addEventListener('mousedown', handleClickOutside);
    return () => {
      document.removeEventListener('mousedown', handleClickOutside);
    };
  }, [openConnectionMenu]);

  useEffect(() => {
    const selectedId = selectedConnection?.id;
    if (selectedId && selectedId !== previousConnectionRef.current) {
      handleSelectConnection(selectedId);
      previousConnectionRef.current = selectedId;
    }
  }, [selectedConnection]);

  useEffect(() => {
    // Check localStorage when component mounts
    const tutorialClosed = localStorage.getItem("isTutorialClosed") === "true";
    setIsTutorialClosed(tutorialClosed);
  }, []);

  const handleToggleExpand = useCallback(() => {
    // Track sidebar toggled event
    analyticsService.trackSidebarToggled(!isExpanded, user?.id || '', user?.username || '');
    
    onToggleExpand();
  }, [isExpanded, onToggleExpand]);

  const handleLogoutClick = () => {
    setShowLogoutConfirm(true);
  };

  const handleLogoutConfirm = () => {
    // If user exists, track logout event
    if (user) {
      analyticsService.trackLogout(user.id, user.username);
    }
    
    // Clear the URL by navigating to root
    window.location.href = '/';
    onLogout();
    setShowLogoutConfirm(false);
  };

  const handleEditConnection = (connection: Chat) => {
    setOpenConnectionMenu(null);
    
    // Track connection edit event
    analyticsService.trackConnectionEdited(
      connection.id,
      connection.connection.type,
      connection.connection.database,
      user?.id || '',
      user?.username || ''
    );
    
    handleSelectConnection(connection.id);
    
    if (onEditConnection) {
      onEditConnection();
    }
  };

  const handleDeleteClick = useCallback((connection: Chat) => {
    setOpenConnectionMenu(null);
    setConnectionToDelete(connection);
  }, []);

  const handleDeleteConfirm = async (chatId: string) => {
    try {
      const connectionToDelete = connections.find(chat => chat.id === chatId);
      
      if (connectionToDelete) {
        // Track connection deleted event
        analyticsService.trackConnectionDeleted(
          chatId,
          connectionToDelete.connection.type,
          connectionToDelete.connection.database,
          user?.id || '',
          user?.username || ''
        );
      }
      
      await chatService.deleteChat(chatId);
      if (onDeleteConnection) {
        onDeleteConnection(chatId);
      }
      setConnectionToDelete(null);
    } catch (error: any) {
      toast.error(error.message, {
        style: {
          background: '#ff4444',
          color: '#fff',
          border: '4px solid #cc0000',
          borderRadius: '12px',
          boxShadow: '4px 4px 0px 0px rgba(0,0,0,1)',
        },
      });
    }
  };

  const handleSelectConnection = useCallback(async (id: string) => {
    try {
      console.log('handleSelectConnection happened', { id, currentConnectedChatId });

      // Don't return early if it's the same connection - we might need to reconnect
      // if (id === currentConnectedChatId) {
      //   return;
      // }

      // Track connection selected event
      const connection = connections.find(chat => chat.id === id);
      if (connection) {
        analyticsService.trackConnectionSelected(
          id,
          connection.connection.type,
          connection.connection.database,
          user?.id || '',
          user?.username || ''
        );
      }
      
      setCurrentConnectedChatId(id);
      onSelectConnection(id);
      onConnectionStatusChange?.(id, false, 'sidebar-connecting');

      // Auto-collapse sidebar on mobile devices
      if (window.innerWidth < 768 && isExpanded) {
        onToggleExpand();
      }

    } catch (error) {
      console.error('Failed to setup connection:', error);
      onConnectionStatusChange?.(id, false, 'sidebar-select-connection');
      toast.error('Failed to connect to database');
    }
  }, [currentConnectedChatId, onSelectConnection, onConnectionStatusChange, connections, isExpanded, onToggleExpand]);

  const handleOpenMenu = (e: React.MouseEvent, connectionId: string) => {
    e.preventDefault();
    e.stopPropagation();
    
    // Get the position of the button
    const button = e.currentTarget;
    const rect = button.getBoundingClientRect();
    
    // Set the position for the dropdown
    setMenuPosition({
      top: rect.top,
      left: rect.right + 10 // Position to the right of the button
    });
    
    // Toggle the menu
    setOpenConnectionMenu(openConnectionMenu === connectionId ? null : connectionId);
  };

  const handleCloseTutorial = (e: React.MouseEvent) => {
    e.stopPropagation(); // Prevent triggering the parent button's onClick
    localStorage.setItem("isTutorialClosed", "true");
    setIsTutorialClosed(true);
  };

  const handleDuplicateConnection = (connection: Chat) => {
    setOpenConnectionMenu(null);
    setConnectionToDuplicate(connection);
  };

  const handleCopyLink = (chatId: string) => {
    const url = `${window.location.origin}/chat/${chatId}`;
    navigator.clipboard.writeText(url).then(() => {
      toast.success('Link copied to clipboard!', {
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
        icon: '📋',
        duration: 2000,
      });
    }).catch(() => {
      toast.error('Failed to copy link', {
        style: {
          background: '#ff4444',
          color: '#fff',
          border: '4px solid #cc0000',
          borderRadius: '12px',
          boxShadow: '4px 4px 0px 0px rgba(0,0,0,1)',
        },
      });
    });
  };

  const handleDuplicateConfirm = async (chatId: string, duplicateMessages: boolean) => {
    try {
      const connectionToDuplicate = connections.find(chat => chat.id === chatId);
      
      if (connectionToDuplicate) {
        // Track connection duplicate event in analytics
        analyticsService.trackConnectionDuplicated(
          chatId,
          connectionToDuplicate.connection.type,
          connectionToDuplicate.connection.database,
          duplicateMessages,
          user?.id || '',
          user?.username || ''
        );
      }

      // Call the duplicateChat service and get the duplicated chat
      const duplicatedChat = await chatService.duplicateChat(chatId, duplicateMessages);
      
      // Call the onDuplicateConnection callback with the ID of the duplicated chat
      if (onDuplicateConnection) {
        onDuplicateConnection(duplicatedChat.id);
      }
      
      toast.success('Chat duplicated successfully!', {
        style: {
          background: '#000',
          color: '#fff',
          border: '4px solid #000',
          borderRadius: '12px',
          boxShadow: '4px 4px 0px 0px rgba(0,0,0,1)',
        },
      });
      
      setConnectionToDuplicate(null);
    } catch (error: any) {
      toast.error(error.message, {
        style: {
          background: '#ff4444',
          color: '#fff',
          border: '4px solid #cc0000',
          borderRadius: '12px',
          boxShadow: '4px 4px 0px 0px rgba(0,0,0,1)',
        },
      });
    }
  };

  // Group connections by date
  const groupedConnections = groupConnectionsByDate(connections);
  const sortedGroups = getSortedGroups(groupedConnections);

  return (
    <>
      <div
        className={`${isExpanded ? 'w-80' : 'w-20'
          } fixed left-0 top-0 h-screen bg-white border-r-4 border-black flex flex-col transition-all duration-300 group z-40 transform md:translate-x-0 ${isExpanded ? 'translate-x-0' : '-translate-x-full md:translate-x-0'
          }`}
      >
        <div className="h-16 px-4 border-b-4 border-black shadow-sm flex justify-between items-center relative hidden md:flex">
          <div className="flex items-center gap-3 relative">
            <Boxes className="w-8 h-8" />
            <h1 className={`text-2xl font-bold tracking-tight transition-opacity duration-300 ${isExpanded ? 'opacity-100' : 'opacity-0 w-0'
              }`}>NeoBase</h1>
          </div>
        </div>

        <div className="flex-1 overflow-y-auto p-4 mt-16 md:mt-0">
          {isLoadingConnections ? (
            <div className="flex items-center justify-center h-full">
              <Loader2 className="w-8 h-8 animate-spin" />
            </div>
          ) : (
            <>
              {connections.length > 0 ? (
                <>
                  {sortedGroups.map(([groupName, groupConnections]) => (
                    <div key={groupName} className={`${isExpanded ? 'mb-6' : 'mb-2'}`}>
                      {isExpanded && (
                        <>
                        <h2 className="text-sm font-semibold text-gray-500 mb-2">{groupName}</h2>
                        </>
                      )}
                      <div className="space-y-4">
                        {groupConnections.map((connection) => (
                          <div key={connection.id} className="relative group">
                            <button
                              onClick={() => handleSelectConnection(connection.id)}
                              className={`w-full h-full cursor-pointer ${isExpanded ? 'p-4' : 'p-3'} rounded-lg transition-all ${selectedConnection?.id === connection.id ? 'bg-[#FFDB58]' : 'bg-white hover:bg-gray-50'
                                }`}
                              title={connection.connection.database}
                            >
                              <div className={`flex items-center h-full ${isExpanded ? 'gap-3' : 'justify-center'}`}>
                                <DatabaseLogo
                                  type={connection.connection.type as 'postgresql' | 'yugabytedb' | 'mysql' | 'clickhouse' | 'mongodb' | 'redis' | 'neo4j' | 'spreadsheet'}
                                  size={28}
                                  className={`transition-transform ${selectedConnection?.id === connection.id ? 'scale-110' : ''}`}
                                />
                                <div className={`transition-opacity duration-300 ${isExpanded ? 'opacity-100' : 'opacity-0 w-0 overflow-hidden'
                                  }`}>
                                  <div className="text-left">
                                    <h3 className="font-bold text-lg leading-tight">
                                      {connection.connection.is_example_db 
                                        ? 'Sample Database' 
                                        : connection.connection.database.length > 20 
                                          ? connection.connection.database.slice(0, 20) + '...' 
                                          : connection.connection.database}
                                    </h3>
                                    <div className="flex w-full flex-row justify-between items-center gap-4">
                                      <p className="text-gray-600 capitalize text-sm truncate pt-0.5">
                                        {connection.connection.type === 'postgresql' 
                                          ? 'PostgreSQL' 
                                          : connection.connection.type === 'yugabytedb' 
                                            ? 'YugabyteDB' 
                                            : connection.connection.type === 'mysql' 
                                              ? 'MySQL' 
                                              : connection.connection.type === 'clickhouse' 
                                                ? 'ClickHouse' 
                                                : connection.connection.type === 'mongodb' 
                                                  ? 'MongoDB' 
                                                  : connection.connection.type === 'redis' 
                                                    ? 'Redis' 
                                                    : connection.connection.type === 'neo4j' 
                                                      ? 'Neo4j' 
                                                      : connection.connection.type === 'spreadsheet'
                                                        ? 'Spreadsheet'
                                                        : 'Unknown'}
                                      </p>
                                      <div className="flex flex-row items-center gap-1.5 mt-1">
                                        <Clock className="w-3.5 h-3.5 text-gray-500" />
                                        <p className="text-right text-gray-500 text-xs whitespace-nowrap ml-auto">
                                          {getRelativeTime(connection.updated_at)}</p>
                                      </div>
                                    </div>
                                  </div>
                                </div>
                              </div>
                            </button>
                            {isExpanded && (
                              <div className="connection-menu-container absolute right-2 top-1/2 -translate-y-1/2">
                                <button
                                  onClick={(e) => handleOpenMenu(e, connection.id)}
                                  className="p-2 opacity-0 group-hover:opacity-100 transition-all neo-border bg-white hover:bg-gray-100"
                                  title="Connection menu"
                                >
                                  <MoreVertical className="w-4 h-4" />
                                </button>
                              </div>
                            )}
                          </div>
                        ))}
                      </div>
                      {isExpanded && ( <div className="h-px bg-gray-200 mt-2"></div> )}
                    </div>
                  ))}
                </>
              ) : (
                isExpanded && (
                  <div className="flex flex-col items-center justify-center h-full bg-neo-gray rounded-lg p-4">
                    <p className="text-black font-bold text-lg">No connections yet</p>
                    <p className="text-gray-600">Add a connection to get started</p>
                  </div>
                )
              )}
            </>
          )}
        </div>
          {/* How to Use Card - Moved to bottom */}
           {isExpanded && !isTutorialClosed && (
                    <div className="px-4">
            <div className="mb-3 relative">
              <button
                onClick={() => {
                  trackViewGuideTutorial(user?.id || '', user?.username || '');
                   setShowDemoModal(true);
                }}
                className="w-full p-4 rounded-lg bg-purple-100 hover:bg-purple-200 transition-colors border-2 border-purple-300 flex items-center gap-3"
              >
                <div className="bg-purple-500 rounded-full p-2">
                  <HelpCircle className="w-5 h-5 text-white" />
                </div>
                <div className="text-left">
                  <h3 className="font-bold text-lg leading-tight text-base">How to Use NeoBase</h3>
                  <p className="text-gray-600 text-sm">Try this quick tutorial</p>
                </div>
              </button>
              {/* Close button */}
              <button
                onClick={handleCloseTutorial}
                className="absolute top-1.5 right-1.5 p-1 rounded-full hover:bg-purple-200 transition-colors"
                aria-label="Close tutorial"
              >
                <X className="w-4 h-4 text-purple-700" />
              </button>
            </div>
            </div>
          )}
          
        <div className="p-4 border-t-4 border-black">

          <div className="hidden md:flex md:flex-col md:space-y-3">
            <button
              onClick={onAddConnection}
              className={`neo-button flex items-center justify-center ${isExpanded ? 'w-full' : 'w-12 h-12 p-3'}`}
              title="Add Connection"
            >
              <Plus className="w-5 h-5" />
              <span className={`transition-opacity duration-300 ${isExpanded ? 'opacity-100 ml-2' : 'opacity-0 w-0 overflow-hidden'}`}>
                Add Connection
              </span>
            </button>

            {isExpanded && (
              <div className="p-3 border-2 border-black rounded-lg bg-white shadow-neo">
                <div className="flex flex-col space-y-2">
                  <div className="flex items-center justify-between">
                    <span className="font-bold text-base">{user != null && user?.username!.length > 16 ? user?.username!.slice(0, 16) + '...' : user?.username}</span>
                    <span className="text-xs text-gray-500 bg-gray-100 px-2 py-1 rounded-full">
                      Joined {formatDate(user?.created_at || '')}
                    </span>
                  </div>
                  <div className="border-t border-gray-200 my-2"></div>
                  <button
                    onClick={handleLogoutClick}
                    className="text-red-500 hover:text-red-700 text-sm font-medium flex items-center gap-2 transition-colors w-fit"
                  >
                    <span>Logout</span>
                    <ArrowRight className="w-4 h-4" />
                  </button>
                </div>
              </div>
            )}
          </div>

          <div className={`flex flex-col gap-3 md:hidden ${!isExpanded && 'hidden'}`}>

            
            <button
              onClick={onAddConnection}
              className="neo-button h-12 flex items-center justify-center"
              title="Add Connection"
            >
              <Plus className="w-5 h-5" />
              <span className="ml-2">Add Connection</span>
            </button>

            <div className="p-3 border-2 border-black rounded-lg bg-white shadow-neo">
              <div className="flex flex-col space-y-2">
                <div className="flex items-center justify-between">
                  <span className="font-bold text-base">{user?.username}</span>
                  <span className="text-xs text-gray-600 bg-gray-100 px-2 py-1 rounded-full">
                    Joined {formatDate(user?.created_at || '')}
                  </span>
                </div>
                <div className="border-t border-gray-200 my-2"></div>
                <button
                  onClick={handleLogoutClick}
                  className="text-red-500 hover:text-red-800 text-sm font-medium flex items-center gap-2 transition-colors"
                >
                  <span>Sign out</span>
                  <ArrowRight className="w-4 h-4" />
                </button>
              </div>
            </div>
          </div>
        </div>

        <button
          onClick={handleToggleExpand}
          className="absolute top-1/2 -translate-y-1/2 -right-4 p-2 bg-white hover:bg-neo-gray rounded-lg transition-colors border-2 border-black"
          title={isExpanded ? "Collapse sidebar" : "Expand sidebar"}
        >
          {isExpanded ? (
            <PanelLeftClose className="w-5 h-5" />
          ) : (
            <PanelLeft className="w-5 h-5" />
          )}
        </button>
      </div>

      {showLogoutConfirm && (
        <div className="fixed inset-0 z-[100] flex items-center justify-center bg-black/50">
          <ConfirmationModal
            title="Confirm Logout"
            message="Are you sure you want to logout? Any unsaved changes will be lost."
            onConfirm={async () => {
              await handleLogoutConfirm();
              setShowLogoutConfirm(false);
            }}
            onCancel={() => setShowLogoutConfirm(false)}
          />
        </div>
      )}

      {connectionToDelete && (
        <div className="fixed inset-0 z-[100] flex items-center justify-center bg-black/50">
          <DeleteConnectionModal
            connectionName={connectionToDelete.connection.is_example_db ? "Sample Database" : connectionToDelete.connection.database}
            chatId={connectionToDelete.id}
            onConfirm={handleDeleteConfirm}
            onCancel={() => setConnectionToDelete(null)}
          />
        </div>
      )}

      {connectionToDuplicate && (
        <div className="fixed inset-0 z-[100] flex items-center justify-center bg-black/50">
          <DuplicateChatModal
            chatName={connectionToDuplicate.connection.database}
            chatId={connectionToDuplicate.id}
            onConfirm={handleDuplicateConfirm}
            onCancel={() => setConnectionToDuplicate(null)}
          />
        </div>
      )}

      {openConnectionMenu && menuPosition && (
        <div 
          className="fixed w-48 bg-white border-4 border-black rounded-lg shadow-[4px_4px_0px_0px_rgba(0,0,0,1)] z-[100] connection-dropdown-menu"
          style={{
            top: `${menuPosition.top}px`,
            left: `${menuPosition.left}px`,
            transform: 'none'
          }}
          onClick={(e) => e.stopPropagation()}
        >
          <div className="py-1">
          <button 
              onClick={() => {
                if (openConnectionMenu) {
                  handleCopyLink(openConnectionMenu);
                }
                setOpenConnectionMenu(null);
                setMenuPosition(null);
              }}
              className="flex items-center w-full text-left px-4 py-2 text-sm font-semibold text-black hover:bg-neo-gray transition-colors"
            >
              <Link className="w-4 h-4 mr-2 text-black" />
              Copy Link
            </button>
            <div className="h-px bg-gray-200 mx-2"></div>
            <button 
              onClick={() => {
                const connection = connections.find(c => c.id === openConnectionMenu);
                if (connection) {
                  handleEditConnection(connection);
                }
                setOpenConnectionMenu(null);
                setMenuPosition(null);
              }}
              className="flex items-center w-full text-left px-4 py-2 text-sm font-semibold text-black hover:bg-neo-gray transition-colors"
            >
              <Pencil className="w-4 h-4 mr-2 text-black" />
              Edit Connection
            </button>
            <div className="h-px bg-gray-200 mx-2"></div>
            <button 
              onClick={() => {
                const connection = connections.find(c => c.id === openConnectionMenu);
                if (connection) {
                  handleDuplicateConnection(connection);
                }
                setOpenConnectionMenu(null);
                setMenuPosition(null);
              }}
              className="flex items-center w-full text-left px-4 py-2 text-sm font-semibold text-black hover:bg-neo-gray transition-colors"
            >
              <Copy className="w-4 h-4 mr-2 text-black" />
              Duplicate Chat
            </button>
            <div className="h-px bg-gray-200 mx-2"></div>
            <button 
              onClick={() => {
                const connection = connections.find(c => c.id === openConnectionMenu);
                if (connection) {
                  handleDeleteClick(connection);
                }
                setOpenConnectionMenu(null);
                setMenuPosition(null);
              }}
              className="flex items-center w-full text-left px-4 py-2 text-sm font-semibold text-red-500 hover:bg-neo-error hover:text-white transition-colors"
            >
              <Trash2 className="w-4 h-4 mr-2" />
              Delete Connection
            </button>
          </div>
        </div>
      )}

      {/* Demo Modal */}
      <DemoModal isOpen={showDemoModal} onClose={() => setShowDemoModal(false)} />
    </>
  );
}