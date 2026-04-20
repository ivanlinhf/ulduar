import type { PresentationGenerationCapabilitiesResponse } from "../../../lib/api";
import { NewMenu } from "../../chat/components/NewMenu";
import { usePresentationWorkspace } from "../usePresentationWorkspace";
import { PresentationComposer } from "./PresentationComposer";
import { PresentationTimeline } from "./PresentationTimeline";

type PresentationWorkspaceProps = {
  capabilities: PresentationGenerationCapabilitiesResponse;
  isImageGenerationEnabled: boolean;
  isImageGenerationAvailable: boolean;
  isPresentationGenerationEnabled: boolean;
  isPresentationGenerationAvailable: boolean;
  onNewChat: () => void;
  onNewImage: () => void;
  onNewPresentation: () => void;
};

export function PresentationWorkspace({
  capabilities,
  isImageGenerationEnabled,
  isImageGenerationAvailable,
  isPresentationGenerationEnabled,
  isPresentationGenerationAvailable,
  onNewChat,
  onNewImage,
  onNewPresentation,
}: PresentationWorkspaceProps) {
  const workspace = usePresentationWorkspace(capabilities);

  return (
    <section className="chat-panel image-panel presentation-panel">
      <header className="chat-header">
        <p className="chat-subtitle">{workspace.workspaceSubtitle}</p>

        <div className="chat-header-actions">
          <NewMenu
            isImageGenerationEnabled={isImageGenerationEnabled}
            isImageGenerationAvailable={isImageGenerationAvailable}
            isPresentationGenerationEnabled={isPresentationGenerationEnabled}
            isPresentationGenerationAvailable={isPresentationGenerationAvailable}
            onNewChat={onNewChat}
            onNewImage={onNewImage}
            onNewPresentation={onNewPresentation}
          />
        </div>
      </header>

      <PresentationTimeline turns={workspace.turns} />

      <PresentationComposer
        attachmentToast={workspace.attachmentToast}
        attachmentsSupported={workspace.attachmentsSupported}
        attachments={workspace.attachments}
        busy={workspace.busy}
        canSubmit={workspace.canSubmit}
        composerRef={workspace.composerRef}
        fileInputRef={workspace.fileInputRef}
        generateButtonLabel={workspace.generateButtonLabel}
        inputAccept={workspace.inputAccept}
        onFileSelection={workspace.handleFileSelection}
        onOpenFilePicker={workspace.openFilePicker}
        onPromptChange={workspace.handlePromptChange}
        onPromptKeyDown={workspace.handlePromptKeyDown}
        onRemoveAttachment={workspace.removeAttachment}
        onSubmit={workspace.handleSubmit}
        onThemePresetChange={workspace.handleThemePresetChange}
        prompt={workspace.prompt}
        screenError={workspace.screenError}
        submissionState={workspace.submissionState}
        themePickerVisible={workspace.themePickerVisible}
        themePresetId={workspace.themePresetId}
        themePresets={workspace.themePresets}
      />
    </section>
  );
}
