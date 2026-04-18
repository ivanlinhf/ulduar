import { useState } from "react";

import { ActionTooltip } from "./features/chat/components/ActionTooltip";
import { NewMenu } from "./features/chat/components/NewMenu";
import { attachmentInputAccept } from "./features/chat/constants";
import { IconInfo, IconReload } from "./features/chat/components/icons";
import { ChatComposer } from "./features/chat/components/ChatComposer";
import { ExpandedComposerDialog } from "./features/chat/components/ExpandedComposerDialog";
import { MessageList } from "./features/chat/components/MessageList";
import { ReloadConfirmationDialog } from "./features/chat/components/ReloadConfirmationDialog";
import { useChatApp } from "./features/chat/useChatApp";
import { ImageWorkspace } from "./features/imagegen/components/ImageWorkspace";
import { PresentationWorkspace } from "./features/presentationgen/components/PresentationWorkspace";
import { reloadLosesSessionMessage, useFrontendUpdate } from "./lib/frontendUpdate";
import { isImageGenerationEnabled, isPresentationGenerationEnabled } from "./lib/config";
import { useImageGenerationBootstrap } from "./lib/imageGeneration";
import { usePresentationGenerationBootstrap } from "./lib/presentationGeneration";

export type WorkspaceMode = "chat" | "image" | "presentation";

export default function App() {
  const [workspace, setWorkspace] = useState<WorkspaceMode>("chat");
  const [newImageKey, setNewImageKey] = useState(0);
  const [newPresentationKey, setNewPresentationKey] = useState(0);
  const chat = useChatApp();
  const imageGeneration = useImageGenerationBootstrap(isImageGenerationEnabled);
  const presentationGeneration = usePresentationGenerationBootstrap(isPresentationGenerationEnabled);
  const turnCount = chat.messages.filter((message) => message.role === "user").length;
  const update = useFrontendUpdate(turnCount);
  const isAnyDialogOpen = chat.isExpandedComposerOpen || update.isReloadConfirmationOpen;

  function handleNewChat() {
    setWorkspace("chat");
    chat.startNewChat();
  }

  function handleNewImage() {
    setWorkspace("image");
    setNewImageKey((k) => k + 1);
  }

  function handleNewPresentation() {
    setWorkspace("presentation");
    setNewPresentationKey((k) => k + 1);
  }

  const imageCapabilities =
    imageGeneration.status === "available" ? imageGeneration.capabilities : null;
  const presentationCapabilities =
    presentationGeneration.status === "available" ? presentationGeneration.capabilities : null;

  return (
    <div className="app-shell">
      <div className="app-backdrop app-backdrop-left" />
      <div className="app-backdrop app-backdrop-right" />

      <main
        className="app-frame"
        ref={chat.appFrameRef}
        aria-hidden={isAnyDialogOpen ? "true" : undefined}
        data-workspace={workspace}
        data-image-generation-bootstrap-state={imageGeneration.status}
        data-presentation-generation-bootstrap-state={presentationGeneration.status}
      >
        {workspace === "image" && imageCapabilities ? (
          <ImageWorkspace
            key={newImageKey}
            capabilities={imageCapabilities}
            isImageGenerationEnabled={isImageGenerationEnabled}
            isImageGenerationAvailable={imageGeneration.status === "available"}
            isPresentationGenerationEnabled={isPresentationGenerationEnabled}
            isPresentationGenerationAvailable={presentationGeneration.status === "available"}
            onNewChat={handleNewChat}
            onNewImage={handleNewImage}
            onNewPresentation={handleNewPresentation}
          />
        ) : workspace === "presentation" && presentationCapabilities ? (
          <PresentationWorkspace
            key={newPresentationKey}
            capabilities={presentationCapabilities}
            isImageGenerationEnabled={isImageGenerationEnabled}
            isImageGenerationAvailable={imageGeneration.status === "available"}
            isPresentationGenerationEnabled={isPresentationGenerationEnabled}
            isPresentationGenerationAvailable={presentationGeneration.status === "available"}
            onNewChat={handleNewChat}
            onNewImage={handleNewImage}
            onNewPresentation={handleNewPresentation}
          />
        ) : (
          <section className="chat-panel">
            <header className="chat-header">
              <p className="chat-subtitle">{chat.chatSubtitle}</p>

              <div className="chat-header-actions">
                <ActionTooltip
                  align="right"
                  wrapperClassName="session-info"
                  tooltipClassName="session-info-tooltip"
                  content={
                    <div className="session-info-tooltip-content">
                      <div className="session-info-row">
                        <span>Session ID</span>
                        <strong>{chat.sessionId || "Pending"}</strong>
                      </div>
                      <div className="session-info-row">
                        <span>Turn count</span>
                        <strong>{turnCount}</strong>
                      </div>
                    </div>
                  }
                >
                  <button
                    aria-label="Session details"
                    className="secondary-button icon-only-button info-button"
                    type="button"
                  >
                    <IconInfo />
                  </button>
                </ActionTooltip>

                <NewMenu
                  isImageGenerationEnabled={isImageGenerationEnabled}
                  isImageGenerationAvailable={imageGeneration.status === "available"}
                  isPresentationGenerationEnabled={isPresentationGenerationEnabled}
                  isPresentationGenerationAvailable={presentationGeneration.status === "available"}
                  onNewChat={handleNewChat}
                  onNewImage={handleNewImage}
                  onNewPresentation={handleNewPresentation}
                />
              </div>
            </header>

            <MessageList
              messages={chat.messages}
              messagesEndRef={chat.messagesEndRef}
              onScroll={chat.handleMessageListScroll}
              transientStatus={chat.transientStatus}
            />

            <ChatComposer
              busy={chat.busy}
              canSubmit={chat.canSubmit}
              composerText={chat.composerText}
              inlineComposerRef={chat.inlineComposerRef}
              onOpenExpandedComposer={chat.openExpandedComposer}
              onOpenFilePicker={chat.openFilePicker}
              onRemoveAttachment={chat.removeAttachment}
              onSubmit={chat.handleSubmit}
              onTextChange={chat.handleComposerTextChange}
              onTextareaKeyDown={chat.handleInlineComposerKeyDown}
              screenError={chat.screenError}
              selectedFiles={chat.selectedFiles}
              submissionState={chat.submissionState}
              submitButtonLabel={chat.submitButtonLabel}
            />
          </section>
        )}
      </main>

      <ExpandedComposerDialog
        busy={chat.busy}
        canSubmit={chat.canSubmit}
        composerText={chat.composerText}
        dialogRef={chat.dialogRef}
        expandedComposerRef={chat.expandedComposerRef}
        isOpen={chat.isExpandedComposerOpen}
        onBackdropMouseDown={chat.handleExpandedBackdropMouseDown}
        onDialogKeyDown={chat.handleExpandedDialogKeyDown}
        onOpenFilePicker={chat.openFilePicker}
        onRemoveAttachment={chat.removeAttachment}
        onSendClick={chat.submitFromExpandedComposer}
        onTextChange={chat.handleComposerTextChange}
        onTextareaKeyDown={chat.handleExpandedComposerKeyDown}
        selectedFiles={chat.selectedFiles}
        submissionState={chat.submissionState}
        submitButtonLabel={chat.submitButtonLabel}
      />

      <ReloadConfirmationDialog
        isOpen={update.isReloadConfirmationOpen}
        onCancel={update.cancelReloadConfirmation}
        onConfirm={update.confirmReloadToUpdate}
      />

      <input
        ref={chat.fileInputRef}
        className="visually-hidden-file-input"
        type="file"
        accept={attachmentInputAccept}
        multiple
        onChange={chat.handleFileSelection}
        disabled={chat.busy}
        tabIndex={-1}
      />

      <div
        className="toast-stack"
        aria-live="polite"
        aria-atomic="true"
        aria-hidden={isAnyDialogOpen ? "true" : undefined}
      >
        {update.updateAvailable ? (
          <div className="toast toast-info toast-with-action">
            <div className="toast-copy">
              <strong>A newer version is available.</strong>
              <span>
                {turnCount > 0
                  ? reloadLosesSessionMessage
                  : "Reload when you're ready to use the latest version."}
              </span>
            </div>

            <ActionTooltip
              align="right"
              side="above"
              wrapperClassName="reload-toast-action"
              content={<span className="action-tooltip-label">Reload</span>}
            >
              <button
                aria-label="Reload"
                className="secondary-button icon-only-button reload-toast-button"
                onClick={update.requestReloadToUpdate}
                type="button"
              >
                <IconReload />
              </button>
            </ActionTooltip>
          </div>
        ) : null}

        {chat.attachmentToast ? (
          <div className="toast toast-warning">
            {chat.attachmentToast}
          </div>
        ) : null}
      </div>
    </div>
  );
}
