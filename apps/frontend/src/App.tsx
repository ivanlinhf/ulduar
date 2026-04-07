import { ActionTooltip } from "./features/chat/components/ActionTooltip";
import { attachmentInputAccept } from "./features/chat/constants";
import { IconInfo, IconNewChat, IconReload } from "./features/chat/components/icons";
import { ChatComposer } from "./features/chat/components/ChatComposer";
import { ExpandedComposerDialog } from "./features/chat/components/ExpandedComposerDialog";
import { MessageList } from "./features/chat/components/MessageList";
import { useChatApp } from "./features/chat/useChatApp";
import { reloadLosesSessionMessage, useFrontendUpdate } from "./lib/frontendUpdate";

const appName = "Ulduar";

export default function App() {
  const chat = useChatApp();
  const turnCount = chat.messages.filter((message) => message.role === "user").length;
  const update = useFrontendUpdate(turnCount);

  return (
    <div className="app-shell">
      <div className="app-backdrop app-backdrop-left" />
      <div className="app-backdrop app-backdrop-right" />

      <main className="app-frame" ref={chat.appFrameRef} aria-hidden={chat.isExpandedComposerOpen ? "true" : undefined}>
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

              <ActionTooltip align="right" content={<span className="action-tooltip-label">New chat</span>}>
                <button
                  aria-label="New chat"
                  className="secondary-button icon-only-button new-chat-button"
                  onClick={chat.startNewChat}
                  type="button"
                >
                  <IconNewChat />
                </button>
              </ActionTooltip>
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

      <div className="toast-stack" aria-live="polite" aria-atomic="true">
        {update.updateAvailable ? (
          <div className="toast toast-info toast-with-action">
            <div className="toast-copy">
              <strong>A newer version of {appName} is available.</strong>
              <span>
                {turnCount > 0
                  ? reloadLosesSessionMessage
                  : "Reload when you're ready to use the latest version."}
              </span>
            </div>

            <button className="secondary-button toast-action-button" onClick={update.reloadToUpdate} type="button">
              <IconReload />
              <span>Reload</span>
            </button>
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
