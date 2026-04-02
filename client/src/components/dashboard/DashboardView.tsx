import { Loader2, Trash2 } from 'lucide-react';
import { useDashboard } from '../../hooks/useDashboard';
import ConfirmationModal from '../modals/ConfirmationModal';
import DashboardEmptyState from './DashboardEmptyState';
import DashboardHeader from './DashboardHeader';
import {
  AddWidgetModal,
  BlueprintPickerModal,
  CreateDashboardPromptModal,
  EditWidgetModal,
  ImportDashboardModal,
  RegenerateDashboardModal,
} from './DashboardModals';
import DashboardProgressOverlay from './DashboardProgressOverlay';
import DashboardWidgetGrid from './DashboardWidgetGrid';

interface DashboardViewProps {
  chatId: string;
  streamId: string | null;
  isConnected: boolean;
  onReconnect?: () => Promise<void>;
}

export default function DashboardView({
  chatId,
  streamId,
  isConnected,
  onReconnect,
}: Readonly<DashboardViewProps>) {
  const {
    isLoadingList,
    dashboardList,
    activeDashboard,
    isLoadingDashboard,
    isRefreshing,
    individuallyRefreshingWidgets,
    blueprints,
    showBlueprintPicker,
    selectedBlueprintIndices,
    isCreatingFromBlueprints,
    generationProgress,
    generationContext,
    showPromptModal,
    showAddWidgetModal,
    isAddingWidget,
    showRegenerateModal,
    showDeleteDashboardConfirm,
    showImportModal,
    isImporting,
    editingWidgetId,
    isEditingWidget,
    setSelectedBlueprintIndices,
    setShowBlueprintPicker,
    setShowPromptModal,
    setShowAddWidgetModal,
    setEditingWidgetId,
    setShowRegenerateModal,
    setShowDeleteDashboardConfirm,
    setShowImportModal,
    loadDashboard,
    handleRefreshDashboard,
    handleCancelDashboardRefresh,
    handleRefreshIntervalChange,
    handleNewDashboard,
    handleRegenerateDashboard,
    handleDeleteDashboard,
    handleExportDashboard,
    handleImportDashboard,
    handleImportSubmit,
    handleManualRefreshWidget,
    handleCancelWidgetRefresh,
    handleWidgetNextPage,
    handleWidgetPreviousPage,
    handleDeleteWidget,
    handleEditWidget,
    handleAddWidget,
    handleExploreSuggestions,
    handleCreateFromBlueprints,
    handleCreateWithAI,
    handleSubmitAIPrompt,
    handleCancelGeneration,
  } = useDashboard({ chatId, streamId, isConnected, onReconnect });

  // =============================================
  // Render
  // =============================================

  if (isLoadingList) {
    return (
      <div className="flex-1 flex items-center justify-center bg-[#FFDB58]/10 pt-16 md:pt-0">
        <div className="flex items-center gap-2">
          <Loader2 className="w-5 h-5 animate-spin text-black" />
          <span className="text-sm font-medium text-black">Loading dashboards...</span>
        </div>
      </div>
    );
  }

  const showEmptyState = dashboardList.length === 0 && !activeDashboard;

  if (showEmptyState && !generationProgress && !showBlueprintPicker && !showPromptModal) {
    return (
      <div className="flex-1 bg-[#FFDB58]/10 overflow-y-auto pt-16 md:pt-0">
        <DashboardEmptyState
          onExploreSuggestions={handleExploreSuggestions}
          onCreateWithAI={handleCreateWithAI}
          onImportDashboard={handleImportDashboard}
        />

        {showImportModal && (
          <ImportDashboardModal
            isImporting={isImporting}
            onSubmit={handleImportSubmit}
            onClose={() => setShowImportModal(false)}
          />
        )}
      </div>
    );
  }

  const editingWidget = editingWidgetId
    ? activeDashboard?.widgets.find((w) => w.id === editingWidgetId)
    : null;

  return (
    <div className="flex-1 flex flex-col bg-[#FFDB58]/10 overflow-hidden pt-16 md:pt-0">
      {showEmptyState ? (
        <div className="flex-1 overflow-y-auto">
          <DashboardEmptyState
            onExploreSuggestions={handleExploreSuggestions}
            onCreateWithAI={handleCreateWithAI}
            onImportDashboard={handleImportDashboard}
          />
        </div>
      ) : (
        <>
          {/* Header */}
          <DashboardHeader
            dashboardList={dashboardList}
            activeDashboard={activeDashboard}
            isConnected={isConnected}
            isRefreshing={isRefreshing}
            onSelectDashboard={(id) => loadDashboard(id)}
            onRefreshDashboard={() => handleRefreshDashboard(false)}
            onCancelRefresh={handleCancelDashboardRefresh}
            onRefreshIntervalChange={handleRefreshIntervalChange}
            onNewDashboard={handleNewDashboard}
            onAddWidget={() => setShowAddWidgetModal(true)}
            onRegenerateDashboard={() => setShowRegenerateModal(true)}
            onDeleteDashboard={() => setShowDeleteDashboardConfirm(true)}
            onExportDashboard={handleExportDashboard}
            onImportDashboard={handleImportDashboard}
          />

          {/* Widget Grid */}
          <div className="flex-1 overflow-y-auto py-8">
            {isLoadingDashboard && (
              <div className="flex items-center justify-center h-64">
                <div className="flex items-center gap-2">
                  <Loader2 className="w-5 h-5 animate-spin text-black" />
                  <span className="text-sm font-medium text-black">Loading dashboard...</span>
                </div>
              </div>
            )}

            {!isLoadingDashboard && activeDashboard && (
              <DashboardWidgetGrid
                dashboard={activeDashboard}
                onDeleteWidget={handleDeleteWidget}
                onEditWidget={(widgetId) => setEditingWidgetId(widgetId)}
                onRefreshWidget={handleManualRefreshWidget}
                onCancelWidgetRefresh={handleCancelWidgetRefresh}
                onWidgetNextPage={handleWidgetNextPage}
                onWidgetPreviousPage={handleWidgetPreviousPage}
                individuallyRefreshingWidgets={individuallyRefreshingWidgets}
                onAddWidget={() => setShowAddWidgetModal(true)}
              />
            )}
          </div>
        </>
      )}

      {/* === MODALS === */}

      {/* Blueprint Picker */}
      {showBlueprintPicker && blueprints.length > 0 && (
        <BlueprintPickerModal
          blueprints={blueprints}
          selectedIndices={selectedBlueprintIndices}
          isCreating={isCreatingFromBlueprints}
          onToggleSelection={(index) => {
            setSelectedBlueprintIndices((prev) => {
              const next = new Set(prev);
              if (next.has(index)) next.delete(index);
              else next.add(index);
              return next;
            });
          }}
          onCreate={handleCreateFromBlueprints}
          onClose={() => setShowBlueprintPicker(false)}
        />
      )}

      {/* Create with AI Prompt */}
      {showPromptModal && (
        <CreateDashboardPromptModal
          onSubmit={handleSubmitAIPrompt}
          onClose={() => setShowPromptModal(false)}
        />
      )}

      {/* Add Widget */}
      {showAddWidgetModal && (
        <AddWidgetModal
          isAdding={isAddingWidget}
          onSubmit={handleAddWidget}
          onClose={() => setShowAddWidgetModal(false)}
        />
      )}

      {/* Edit Widget */}
      {editingWidgetId && editingWidget && (
        <EditWidgetModal
          widgetTitle={editingWidget.title}
          isEditing={isEditingWidget}
          onSubmit={(prompt) => handleEditWidget(editingWidgetId, prompt)}
          onClose={() => setEditingWidgetId(null)}
        />
      )}

      {/* Regenerate Dashboard */}
      {showRegenerateModal && (
        <RegenerateDashboardModal
          onSubmit={handleRegenerateDashboard}
          onClose={() => setShowRegenerateModal(false)}
        />
      )}

      {/* Delete Dashboard Confirmation */}
      {showDeleteDashboardConfirm && (
        <ConfirmationModal
          icon={<Trash2 className="w-6 h-6 text-neo-error" />}
          title="Delete Dashboard"
          message={`Are you sure you want to delete "${activeDashboard?.name ?? ''}"? All widgets will be permanently removed. This action cannot be undone.`}
          buttonText="Delete"
          onConfirm={handleDeleteDashboard}
          onCancel={() => setShowDeleteDashboardConfirm(false)}
          zIndex="z-[120]"
        />
      )}

      {/* Import Dashboard */}
      {showImportModal && (
        <ImportDashboardModal
          isImporting={isImporting}
          onSubmit={handleImportSubmit}
          onClose={() => setShowImportModal(false)}
        />
      )}

      {/* Generation Progress */}
      {generationProgress && (
        <DashboardProgressOverlay
          progress={generationProgress}
          generationContext={generationContext}
          onCancel={handleCancelGeneration}
        />
      )}
    </div>
  );
}
