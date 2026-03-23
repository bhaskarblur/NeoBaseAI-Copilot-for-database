import axios from 'axios';
import { Boxes } from 'lucide-react';
import { useCallback, useEffect, useState } from 'react';
import toast, { Toaster } from 'react-hot-toast';
import { Routes, Route, useNavigate, useParams, Navigate } from 'react-router-dom';
import AuthForm from './components/auth/AuthForm';
import ChatWindow from './components/chat/ChatWindow';
import { Message } from './types/query';
import StarUsButton from './components/common/StarUsButton';
import SuccessBanner from './components/common/SuccessBanner';
import Sidebar from './components/chat/Sidebar';
import ConnectionModal from './components/modals/ConnectionModal';
import { StreamProvider, useStream } from './contexts/StreamContext';
import { UserProvider, useUser } from './contexts/UserContext';
import authService from './services/authService';
import './services/axiosConfig';
import chatService from './services/chatService';
import analyticsService from './services/analyticsService';
import { LoginFormData, SignupFormData } from './types/auth';
import { Chat, ChatSettings, ChatsResponse, Connection } from './types/chat';
import { SendMessageResponse } from './types/messages';
import WelcomeSection from './components/app/WelcomeSection';
import LoadingComponent from './components/app/Loading';
import GoogleAuthCallback from './pages/GoogleAuthCallback';
import { useSSEConnection } from './hooks/useSSEConnection';
import { useChatActions } from './hooks/useChatActions';

// ─── Toast presets ────────────────────────────────────────────────────────────
const toastStyle = {
    style: {
        background: '#000', color: '#fff', border: '4px solid #000', borderRadius: '12px',
        boxShadow: '4px 4px 0px 0px rgba(0,0,0,1)', padding: '12px 24px',
        fontSize: '16px', fontWeight: '500', zIndex: 9999,
    },
} as const;

const errorToast = {
    style: { ...toastStyle.style, background: '#ff4444', border: '4px solid #cc0000', fontWeight: 'bold' as const },
    duration: 4000,
    icon: '\u26a0\ufe0f',
};

// ─── Inner component (needs router hooks) ─────────────────────────────────────
function AppContent() {
    const navigate = useNavigate();
    const { chatId: chatIdFromUrl } = useParams<{ chatId?: string }>();
    const { user, setUser } = useUser();
    const { generateStreamId } = useStream();

    // ── Core state ────────────────────────────────────────────────────────────
    const [isAuthenticated, setIsAuthenticated] = useState(false);
    const [isLoading, setIsLoading] = useState(true);
    const [hasCheckedAuth, setHasCheckedAuth] = useState(false);
    const [showConnectionModal, setShowConnectionModal] = useState(false);
    const [isEditingConnection, setIsEditingConnection] = useState(false);
    const [isSidebarExpanded, setIsSidebarExpanded] = useState(true);
    const [selectedConnection, setSelectedConnection] = useState<Chat>();
    const [messages, setMessages] = useState<Message[]>([]);
    const [authError, setAuthError] = useState<string | null>(null);
    const [successMessage, setSuccessMessage] = useState<string | null>(null);
    const [chats, setChats] = useState<Chat[]>([]);
    const [isLoadingChats, setIsLoadingChats] = useState(false);
    const [connectionStatuses, setConnectionStatuses] = useState<Record<string, boolean>>({});
    const [isMessageSending, setIsMessageSending] = useState(false);
    const [temporaryMessage, setTemporaryMessage] = useState<Message | null>(null);
    const [refreshSchemaController, setRefreshSchemaController] = useState<AbortController | null>(null);
    const [recoRefreshToken, setRecoRefreshToken] = useState(0);
    const [llmModels, setLlmModels] = useState<any[]>([]);
    const [isLoadingModels, setIsLoadingModels] = useState(false);

    const handleConnectionStatusChange = useCallback((chatId: string, isConnected: boolean, _from?: string) => {
        if (chatId && typeof isConnected === 'boolean') {
            setConnectionStatuses(prev => ({ ...prev, [chatId]: isConnected }));
        }
    }, []);

    // ── SSE hook ──────────────────────────────────────────────────────────────
    const sse = useSSEConnection({
        selectedConnectionId: selectedConnection?.id,
        messages,
        temporaryMessage,
        setMessages,
        setTemporaryMessage,
        setRecoRefreshToken,
        onConnectionStatusChange: handleConnectionStatusChange,
    });

    // ── Chat actions hook ─────────────────────────────────────────────────────
    const chatActions = useChatActions({
        chats, setChats,
        selectedConnection, setSelectedConnection,
        setMessages,
        setSuccessMessage,
        connectionStatuses,
        handleConnectionStatusChange,
        streamId: sse.streamId,
        generateStreamId,
        eventSource: sse.eventSource,
        setEventSource: sse.setEventSource,
        setupSSEConnection: sse.setupSSEConnection,
    });

    // ── Document title ────────────────────────────────────────────────────────
    useEffect(() => {
        if (!isAuthenticated) {
            document.title = 'Login to Continue | NeoBase Dashboard';
        } else if (selectedConnection) {
            const name = selectedConnection.connection.is_example_db
                ? 'Sample Database'
                : selectedConnection.connection.database;
            document.title = `${name} | NeoBase Dashboard`;
        } else {
            document.title = 'NeoBase Dashboard - AI Copilot for database';
        }
    }, [isAuthenticated, selectedConnection]);

    // ── Auth check ────────────────────────────────────────────────────────────
    useEffect(() => {
        if (!hasCheckedAuth) { setHasCheckedAuth(true); checkAuth(); }
    }, [hasCheckedAuth]);

    const checkAuth = async () => {
        try {
            const response = await authService.getUser();
            setIsAuthenticated(response.success);
            if (response.success && response.data) {
                const userData = { id: response.data.id, username: response.data.username, created_at: response.data.created_at };
                setUser(userData);
                analyticsService.identifyUser(userData.id, userData.username, userData.created_at);
            }
        } catch (error: any) {
            setIsAuthenticated(false);
            setAuthError(error.message);
        } finally {
            setIsLoading(false);
        }
    };

    useEffect(() => {
        if (authError) { toast.error(authError, errorToast); setAuthError(null); }
    }, [authError]);

    // ── Load chats + LLM models on auth ───────────────────────────────────────
    useEffect(() => {
        if (!isAuthenticated) return;

        const loadChats = async () => {
            setIsLoadingChats(true);
            try {
                // Load models in parallel
                axios.get(`${import.meta.env.VITE_API_URL}/llm-models`).then(r => {
                    if (r.data.success && r.data.data.models) setLlmModels(r.data.data.models);
                }).catch(() => {}).finally(() => setIsLoadingModels(false));
                setIsLoadingModels(true);

                const response = await axios.get<ChatsResponse>(`${import.meta.env.VITE_API_URL}/chats`, {
                    withCredentials: true,
                    headers: { 'Authorization': `Bearer ${localStorage.getItem('token')}`, 'Accept': 'application/json', 'Content-Type': 'application/json' },
                });
                if (response.data?.data?.chats) {
                    setChats(response.data.data.chats);
                    if (chatIdFromUrl) {
                        const fromUrl = response.data.data.chats.find(c => c.id === chatIdFromUrl);
                        if (fromUrl) { setSelectedConnection(fromUrl); chatActions.handleSelectConnection(chatIdFromUrl); }
                    }
                }
            } catch { /* ignore */ } finally {
                setIsLoadingChats(false);
            }
        };
        loadChats();
    // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [isAuthenticated]);

    // ── Handle chat from URL on subsequent navigation ─────────────────────────
    useEffect(() => {
        if (chatIdFromUrl && chats.length > 0 && !isLoadingChats && !chatActions.isSelectingConnection) {
            const fromUrl = chats.find(c => c.id === chatIdFromUrl);
            if (fromUrl && (!selectedConnection || selectedConnection.id !== chatIdFromUrl)) {
                chatActions.handleSelectConnection(chatIdFromUrl);
            }
        }
    // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [chatIdFromUrl, chats, isLoadingChats, selectedConnection]);

    // ── Auth handlers ─────────────────────────────────────────────────────────
    const handleLogin = async (data: LoginFormData) => {
        const response = await authService.login(data);
        const userData = { id: response.user.id, username: response.user.username, created_at: response.user.created_at };
        setUser(userData);
        setIsAuthenticated(true);
        setSuccessMessage(`Welcome back, ${userData.username}!`);
        analyticsService.trackLogin(userData.id, userData.username);
        analyticsService.identifyUser(userData.id, userData.username, userData.created_at);
    };

    const handleSignup = async (data: SignupFormData) => {
        const response = await authService.signup(data);
        const userData = { id: response.user.id, username: response.user.username, created_at: response.user.created_at };
        setIsAuthenticated(true);
        setUser(userData);
        setSuccessMessage(`Welcome to NeoBase, ${userData.username}!`);
        analyticsService.trackSignup(userData.id, userData.username);
        analyticsService.identifyUser(userData.id, userData.username, userData.created_at);
    };

    const handleLogout = async () => {
        try { authService.logout(); } catch { /* ignore */ }
        setUser(null);
        setSuccessMessage("You've been logged out!");
        setIsAuthenticated(false);
        setSelectedConnection(undefined);
        setMessages([]);
        window.location.href = '/';
    };

    // ── Schema refresh ────────────────────────────────────────────────────────
    const handleRefreshSchema = async () => {
        try {
            const controller = new AbortController();
            setRefreshSchemaController(controller);
            const ok = await chatService.refreshSchema(selectedConnection?.id || '', controller);
            if (ok) toast.success('Knowledge base refreshed successfully');
            else toast.error('Cancelled Knowledge Base Refresh');
        } catch (error) {
            toast.error('Failed to refresh knowledge base: ' + error);
        }
    };

    const handleCancelRefreshSchema = async () => {
        if (refreshSchemaController) { refreshSchemaController.abort(); setRefreshSchemaController(null); }
    };

    // ── Cancel stream ─────────────────────────────────────────────────────────
    const handleCancelStream = async () => {
        if (!selectedConnection?.id || !sse.streamId) return;
        try {
            await chatService.cancelStream(selectedConnection.id, sse.streamId);
            setMessages(prev => prev.filter(m => !m.is_streaming).map(m => ({ ...m, is_streaming: false })));
        } catch { /* ignore */ }
    };

    // ── Send message ──────────────────────────────────────────────────────────
    const handleSendMessage = async (content: string, llmModel?: string) => {
        if (!selectedConnection?.id || !sse.streamId || isMessageSending) return;
        try {
            setIsMessageSending(true);
            await sse.checkSSEConnection();
            await new Promise(resolve => setTimeout(resolve, 100));

            const response = await chatService.sendMessage(selectedConnection.id, 'temp', sse.streamId, content, llmModel);
            if (response.success) {
                const userMessage: Message = {
                    id: response.data.id, type: 'user', content: response.data.content,
                    is_loading: false, queries: [], is_streaming: false,
                    created_at: new Date().toISOString(),
                };
                const tempMsg: Message = {
                    id: `temp-${Date.now()}`, type: 'assistant', content: '',
                    queries: [], action_buttons: [], is_loading: true,
                    loading_steps: [{ text: 'NeoBase is analyzing your request..', done: false }],
                    is_streaming: true, created_at: new Date().toISOString(),
                };
                setMessages(prev => [...prev, userMessage, tempMsg]);
                setTemporaryMessage(tempMsg);
                sse.scheduleStreamTimeout(tempMsg.id);
            }
        } catch {
            toast.error('Failed to send message', errorToast);
        } finally {
            setIsMessageSending(false);
        }
    };

    // ── Edit message ──────────────────────────────────────────────────────────
    const handleEditMessage = async (id: string, content: string) => {
        if (!selectedConnection?.id || !sse.streamId || isMessageSending || !content) return;
        try {
            await sse.checkSSEConnection();

            const response = await axios.patch<SendMessageResponse>(
                `${import.meta.env.VITE_API_URL}/chats/${selectedConnection.id}/messages/${id}`,
                { stream_id: sse.streamId, content },
                { withCredentials: true, headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${localStorage.getItem('token')}` } },
            );

            if (response.data.success) {
                setMessages(prev => prev.map(msg =>
                    msg.id === id ? { ...msg, content, is_edited: true, updated_at: new Date().toISOString() } : msg,
                ));

                const aiMessage = messages.find(msg => msg.user_message_id === id);
                if (aiMessage) {
                    setMessages(prev => prev.map(msg =>
                        msg.id === aiMessage.id
                            ? { ...msg, is_edited: true, content: '', queries: [], action_buttons: [], updated_at: new Date().toISOString(), loading_steps: [{ text: 'NeoBase is analyzing your request..', done: false }], is_streaming: true }
                            : msg,
                    ));
                    setTemporaryMessage(messages.find(m => m.id === aiMessage.id) || null);
                } else {
                    const tempMsg: Message = {
                        id: 'temp', type: 'assistant', content: '',
                        queries: [], action_buttons: [], is_loading: true,
                        loading_steps: [{ text: 'NeoBase is analyzing your request..', done: false }],
                        is_streaming: true, created_at: new Date().toISOString(),
                    };
                    setMessages(prev => [...prev, tempMsg]);
                    setTemporaryMessage(tempMsg);
                    sse.scheduleStreamTimeout(tempMsg.id);
                }
            }
        } catch {
            toast.error('Failed to edit message', errorToast);
        }
    };

    const handleEditConnectionFromChatWindow = () => { setIsEditingConnection(true); setShowConnectionModal(true); };

    // ── Connection status cleanup ─────────────────────────────────────────────
    useEffect(() => {
        if (!selectedConnection) setConnectionStatuses({});
    }, [selectedConnection]);

    // ── Render ────────────────────────────────────────────────────────────────
    if (isLoading) return <LoadingComponent />;
    if (!isAuthenticated) return (
        <>
            <AuthForm onLogin={handleLogin} onSignup={handleSignup} />
            <StarUsButton />
        </>
    );

    return (
        <div className="flex flex-col md:flex-row bg-[#FFDB58]/10 min-h-screen">
            {/* Mobile header */}
            <div className={`${!isSidebarExpanded && selectedConnection ? 'hidden' : 'fixed'} md:fixed top-0 left-0 right-0 h-16 bg-white border-b-4 border-black md:hidden z-50 flex items-center justify-between px-4`}>
                <div className="flex items-center gap-2">
                    <Boxes className="w-8 h-8" />
                    <h1 className="text-2xl font-bold">NeoBase</h1>
                </div>
                <div className="block md:hidden">
                    <StarUsButton isMobile className="scale-90" />
                </div>
            </div>

            <div className="hidden md:block">
                <StarUsButton />
            </div>

            <Sidebar
                isExpanded={isSidebarExpanded}
                onToggleExpand={() => setIsSidebarExpanded(prev => !prev)}
                connections={chats}
                isLoadingConnections={isLoadingChats}
                onSelectConnection={chatActions.handleSelectConnection}
                onAddConnection={() => { setIsEditingConnection(false); setShowConnectionModal(true); }}
                onLogout={handleLogout}
                selectedConnection={selectedConnection}
                onDeleteConnection={chatActions.handleDeleteConnection}
                onEditConnection={handleEditConnectionFromChatWindow}
                onDuplicateConnection={chatActions.handleDuplicateConnection}
                onConnectionStatusChange={handleConnectionStatusChange}
                eventSource={sse.eventSource}
            />

            <div className="flex-1 transition-all duration-200 ease-in-out">
                {selectedConnection ? (
                    <ChatWindow
                        chat={selectedConnection}
                        isExpanded={isSidebarExpanded}
                        messages={messages}
                        checkSSEConnection={sse.checkSSEConnection}
                        setMessages={setMessages}
                        onSendMessage={handleSendMessage}
                        onClearChat={chatActions.handleClearChat}
                        onEditMessage={handleEditMessage}
                        onCloseConnection={chatActions.handleCloseConnection}
                        onEditConnection={chatActions.handleEditConnection}
                        onConnectionStatusChange={handleConnectionStatusChange}
                        isConnected={!!connectionStatuses[selectedConnection.id]}
                        onCancelStream={handleCancelStream}
                        onRefreshSchema={handleRefreshSchema}
                        onCancelRefreshSchema={handleCancelRefreshSchema}
                        onUpdateSelectedCollections={chatActions.handleUpdateSelectedCollections}
                        onEditConnectionFromChatWindow={handleEditConnectionFromChatWindow}
                        userId={user?.id || ''}
                        userName={user?.username || ''}
                        recoVersion={recoRefreshToken}
                        llmModels={llmModels}
                        isLoadingModels={isLoadingModels}
                    />
                ) : (
                    <WelcomeSection
                        isSidebarExpanded={isSidebarExpanded}
                        setShowConnectionModal={setShowConnectionModal}
                        toastStyle={toastStyle}
                    />
                )}
            </div>

            {showConnectionModal && (
                <ConnectionModal
                    onClose={updatedChat => {
                        setShowConnectionModal(false);
                        setIsEditingConnection(false);
                        if (updatedChat) {
                            setChats(prev => prev.map(c => c.id === updatedChat.id ? updatedChat : c));
                            if (selectedConnection?.id === updatedChat.id) setSelectedConnection(updatedChat);
                        }
                    }}
                    onSubmit={chatActions.handleAddConnection}
                    onUpdateSelectedCollections={chatActions.handleUpdateSelectedCollections}
                    onRefreshSchema={handleRefreshSchema}
                    initialData={isEditingConnection ? selectedConnection : undefined}
                    onEdit={isEditingConnection ? async (connection, settings) => {
                        try {
                            const updated = await chatService.editChat(selectedConnection!.id, connection as Connection, settings as ChatSettings);
                            setChats(prev => prev.map(c => c.id === updated.id ? updated : c));
                            if (selectedConnection?.id === updated.id) setSelectedConnection(updated);
                            toast.success('Connection updated successfully!', toastStyle);
                            return { success: true, updatedChat: updated };
                        } catch (error: any) {
                            toast.error(error.message, errorToast);
                            return { success: false, error: error.message };
                        }
                    } : undefined}
                />
            )}

            <Toaster
                position="bottom-center"
                reverseOrder={false}
                gutter={8}
                containerClassName="!fixed"
                containerStyle={{ zIndex: 9999, bottom: '2rem' }}
                toastOptions={{
                    success: { style: toastStyle.style, duration: 2000, icon: '\ud83d\udc4b' },
                    error: { style: { ...toastStyle.style, background: '#ff4444', border: '4px solid #cc0000', fontWeight: 'bold' }, duration: 4000, icon: '\u26a0\ufe0f' },
                }}
            />

            {successMessage && <SuccessBanner message={successMessage} onClose={() => setSuccessMessage(null)} />}
        </div>
    );
}

// ─── Root App ─────────────────────────────────────────────────────────────────
function App() {
    useEffect(() => {
        try { analyticsService.initAnalytics(); } catch { /* ignore */ }
    }, []);

    return (
        <UserProvider>
            <StreamProvider>
                <Routes>
                    <Route path="/" element={<AppContent />} />
                    <Route path="/chat/:chatId" element={<AppContent />} />
                    <Route path="/auth/google/callback" element={<GoogleAuthCallback />} />
                    <Route path="*" element={<Navigate to="/" replace />} />
                </Routes>
            </StreamProvider>
        </UserProvider>
    );
}

export default App;
