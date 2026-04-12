import { NewMenu } from "../../chat/components/NewMenu";
import type { ImageGenerationCapabilitiesResponse } from "../../../lib/api";
import { useImageWorkspace } from "../useImageWorkspace";
import { ImageComposer } from "./ImageComposer";
import { ImageTimeline } from "./ImageTimeline";

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

      <ImageTimeline turns={workspace.turns} />

      <ImageComposer
        attachmentToast={workspace.attachmentToast}
        busy={workspace.busy}
        canSubmit={workspace.canSubmit}
        capabilities={capabilities}
        composerRef={workspace.composerRef}
        fileInputRef={workspace.fileInputRef}
        onFileSelection={workspace.handleFileSelection}
        onOpenFilePicker={workspace.openFilePicker}
        onPromptChange={workspace.handlePromptChange}
        onPromptKeyDown={workspace.handlePromptKeyDown}
        onRemoveReferenceImage={workspace.removeReferenceImage}
        onResolutionChange={workspace.handleResolutionChange}
        onReuseImage={workspace.reuseImage}
        onSubmit={workspace.handleSubmit}
        mode={workspace.mode}
        prompt={workspace.prompt}
        referenceImages={workspace.referenceImages}
        reusableImages={workspace.reusableImages}
        resolution={workspace.resolution}
        reusingImageIds={workspace.reusingImageIds}
        screenError={workspace.screenError}
        submissionState={workspace.submissionState}
        generateButtonLabel={workspace.generateButtonLabel}
      />
    </section>
  );
}
