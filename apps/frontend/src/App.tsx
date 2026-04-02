import { ChatComposer } from "./features/chat/components/ChatComposer";
import { ChatHero } from "./features/chat/components/ChatHero";
import { ExpandedComposerDialog } from "./features/chat/components/ExpandedComposerDialog";
import { MessageList } from "./features/chat/components/MessageList";
import { useChatApp } from "./features/chat/useChatApp";

export default function App() {
  const chat = useChatApp();

  return (
    <div className="app-shell">
      <div className="app-backdrop app-backdrop-left" />
      <div className="app-backdrop app-backdrop-right" />

      <main className="app-frame" ref={chat.appFrameRef} aria-hidden={chat.isExpandedComposerOpen ? "true" : undefined}>
        <ChatHero
          bootstrapState={chat.bootstrapState}
          messageCount={chat.messages.length}
          onNewChat={chat.startNewChat}
          sessionId={chat.sessionId}
        />

        <section className="chat-panel">
          <header className="chat-header">
            <p className="chat-subtitle">{chat.chatSubtitle}</p>
          </header>

          <MessageList
            messages={chat.messages}
            messagesEndRef={chat.messagesEndRef}
            onScroll={chat.handleMessageListScroll}
          />

          <ChatComposer
            attachmentSummary={chat.attachmentSummary}
            busy={chat.busy}
            canSubmit={chat.canSubmit}
            composerText={chat.composerText}
            fileInputRef={chat.fileInputRef}
            inlineComposerRef={chat.inlineComposerRef}
            onFileSelection={chat.handleFileSelection}
            onOpenExpandedComposer={chat.openExpandedComposer}
            onOpenFilePicker={chat.openFilePicker}
            onRemoveAttachment={chat.removeAttachment}
            onSubmit={chat.handleSubmit}
            onTextChange={chat.handleComposerTextChange}
            onTextareaKeyDown={chat.handleInlineComposerKeyDown}
            screenError={chat.screenError}
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
        onSendClick={chat.submitFromExpandedComposer}
        onTextChange={chat.handleComposerTextChange}
        onTextareaKeyDown={chat.handleExpandedComposerKeyDown}
        submissionState={chat.submissionState}
        submitButtonLabel={chat.submitButtonLabel}
      />
    </div>
  );
}
