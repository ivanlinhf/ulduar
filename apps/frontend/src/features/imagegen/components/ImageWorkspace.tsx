import { NewMenu } from "../../chat/components/NewMenu";
import type { ImageGenerationCapabilitiesResponse } from "../../../lib/api";
import { referenceImageInputAccept } from "../constants";
import { useImageWorkspace } from "../useImageWorkspace";
import { ImageComposer } from "./ImageComposer";

type ImageWorkspaceProps = {
  capabilities: ImageGenerationCapabilitiesResponse;
  isImageGenerationEnabled: boolean;
  isImageGenerationAvailable: boolean;
  onNewChat: () => void;
  onNewImage: () => void;
};

export function ImageWorkspace({
  capabilities,
  isImageGenerationEnabled,
  isImageGenerationAvailable,
  onNewChat,
  onNewImage,
}: ImageWorkspaceProps) {
  const workspace = useImageWorkspace(capabilities);

  return (
    <>
      <section className="chat-panel image-panel">
        <header className="chat-header">
          <p className="chat-subtitle">{workspace.workspaceSubtitle}</p>

          <div className="chat-header-actions">
            <NewMenu
              isImageGenerationEnabled={isImageGenerationEnabled}
              isImageGenerationAvailable={isImageGenerationAvailable}
              onNewChat={onNewChat}
              onNewImage={onNewImage}
            />
          </div>
        </header>

        <div className="image-workspace-body" />

        <ImageComposer
          busy={workspace.busy}
          canSubmit={workspace.canSubmit}
          capabilities={capabilities}
          composerRef={workspace.composerRef}
          onOpenFilePicker={workspace.openFilePicker}
          onPromptChange={workspace.handlePromptChange}
          onPromptKeyDown={workspace.handlePromptKeyDown}
          onRemoveReferenceImage={workspace.removeReferenceImage}
          onResolutionChange={workspace.handleResolutionChange}
          onSubmit={workspace.handleSubmit}
          prompt={workspace.prompt}
          referenceImages={workspace.referenceImages}
          resolution={workspace.resolution}
          screenError={workspace.screenError}
          submissionState={workspace.submissionState}
          generateButtonLabel={workspace.generateButtonLabel}
        />

        <input
          ref={workspace.fileInputRef}
          className="visually-hidden-file-input"
          type="file"
          accept={referenceImageInputAccept}
          multiple
          onChange={workspace.handleFileSelection}
          disabled={workspace.busy}
          tabIndex={-1}
        />
      </section>

      {workspace.attachmentToast ? (
        <div className="toast-stack image-workspace-toast-stack" aria-live="polite" aria-atomic="true">
          <div className="toast toast-warning">{workspace.attachmentToast}</div>
        </div>
      ) : null}
    </>
  );
}
