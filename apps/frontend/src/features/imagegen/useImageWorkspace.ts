import {
  startTransition,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ChangeEvent,
  type KeyboardEvent,
  type SubmitEvent,
} from "react";

import {
  createImageGeneration,
  createSession,
  streamImageGeneration,
  type ImageGenerationCapabilitiesResponse,
  type ImageGenerationMode,
} from "../../lib/api";
import { apiBaseURL } from "../../lib/config";
import { imageToastDurationMs } from "./constants";
import type {
  ImageBootstrapState,
  ImageSubmissionState,
  ImageTurn,
  ImageTurnOutputImage,
  ImageTurnReferenceImage,
  ReusableImageSource,
  SelectedReferenceImage,
} from "./types";
import { createLocalId, toErrorMessage, validateReferenceImages } from "./utils";

export function useImageWorkspace(capabilities: ImageGenerationCapabilitiesResponse) {
  const defaultResolution = capabilities.resolutions[0]?.key ?? "";

  const [bootstrapState, setBootstrapState] = useState<ImageBootstrapState>("idle");
  const [submissionState, setSubmissionState] = useState<ImageSubmissionState>("idle");
  const [sessionId, setSessionId] = useState("");
  const [prompt, setPrompt] = useState("");
  const [resolution, setResolution] = useState(defaultResolution);
  const [referenceImages, setReferenceImages] = useState<SelectedReferenceImage[]>([]);
  const [reusingImageIds, setReusingImageIds] = useState<string[]>([]);
  const [screenError, setScreenError] = useState("");
  const [attachmentToast, setAttachmentToast] = useState("");
  const [turns, setTurns] = useState<ImageTurn[]>([]);

  const streamCleanupRef = useRef<(() => void) | null>(null);
  const composerRef = useRef<HTMLTextAreaElement | null>(null);
  const fileInputRef = useRef<HTMLInputElement | null>(null);
  const attachmentToastTimeoutRef = useRef<number | null>(null);
  const referenceImagesRef = useRef<SelectedReferenceImage[]>([]);

  const maxReferenceImages = capabilities.maxReferenceImages;
  const hasReferenceImages = referenceImages.length > 0;
  const supportsImageEdit = capabilities.modes.includes("image_edit");
  const supportsTextToImage = capabilities.modes.includes("text_to_image");
  const mode: ImageGenerationMode =
    hasReferenceImages && supportsImageEdit ? "image_edit" : "text_to_image";
  const busy = bootstrapState === "loading" || submissionState !== "idle";
  const canSubmit =
    prompt.trim() !== "" &&
    !busy &&
    bootstrapState === "ready" &&
    ((hasReferenceImages && supportsImageEdit) || (!hasReferenceImages && supportsTextToImage));

  const generateButtonLabel =
    submissionState === "streaming"
      ? "Generating..."
      : submissionState === "submitting"
        ? "Submitting..."
        : "Generate";

  const workspaceSubtitle =
    submissionState !== "idle"
      ? "Generating image..."
      : bootstrapState === "loading"
        ? "Creating session..."
        : bootstrapState === "error"
          ? "Unable to create session."
          : "Ready to generate.";

  const reusableImages = useMemo<ReusableImageSource[]>(
    () => [
      ...turns.flatMap((turn) =>
        turn.referenceImages
          .filter((image) => image.sourceKind === "upload")
          .map((image) => ({
            id: `${turn.id}:reference:${image.id}`,
            kind: "upload" as const,
            name: image.name,
            mediaType: image.file.type,
            file: image.file,
          })),
      ),
      ...turns.flatMap((turn) =>
        turn.outputImages.map((image) => ({
          id: `${turn.id}:output:${image.assetId}`,
          kind: "generated" as const,
          name: image.filename,
          mediaType: image.mediaType,
          contentUrl: image.contentUrl,
        })),
      ),
    ],
    [turns],
  );

  useEffect(() => {
    referenceImagesRef.current = referenceImages;
  }, [referenceImages]);

  // Bootstrap a session on mount (capabilities are guaranteed non-null by ImageWorkspace).
  useEffect(() => {
    const firstResolution = capabilities.resolutions[0]?.key ?? "";
    setResolution(firstResolution);
    void bootstrapSession();

    return () => {
      if (attachmentToastTimeoutRef.current !== null) {
        window.clearTimeout(attachmentToastTimeoutRef.current);
        attachmentToastTimeoutRef.current = null;
      }
      streamCleanupRef.current?.();
      streamCleanupRef.current = null;
    };
    // bootstrapSession is defined in the same hook scope and only needs to run once on mount
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  async function bootstrapSession() {
    closeStream();
    clearAttachmentToast();
    setBootstrapState("loading");
    setSubmissionState("idle");
    setScreenError("");
    setPrompt("");
    setReferenceImages([]);
    referenceImagesRef.current = [];
    setReusingImageIds([]);
    setSessionId("");
    setTurns([]);

    try {
      const session = await createSession();
      startTransition(() => {
        setSessionId(session.sessionId);
        setBootstrapState("ready");
      });
    } catch (error) {
      setBootstrapState("error");
      setScreenError(toErrorMessage(error, "Failed to create an image session"));
    }
  }

  async function submitGeneration() {
    if (!canSubmit || sessionId === "") {
      return;
    }

    const validationError = validateReferenceImages(
      referenceImagesRef.current.map((referenceImage) => referenceImage.file),
      maxReferenceImages,
    );
    if (validationError) {
      showAttachmentToast(validationError);
      return;
    }

    const draftPrompt = prompt;
    const draftImages = referenceImagesRef.current;
    const draftResolution = resolution;
    const draftMode = mode;
    const turnId = createLocalId("turn");

    setScreenError("");
    setSubmissionState("submitting");
    setPrompt("");
    setReferenceImages([]);
    referenceImagesRef.current = [];

    const pendingTurn: ImageTurn = {
      id: turnId,
      generationId: "",
      prompt: draftPrompt,
      mode: draftMode,
      resolution: draftResolution,
      referenceImages: draftImages.map(({ id, file, sourceKind }): ImageTurnReferenceImage => ({
        id,
        previewUrl: URL.createObjectURL(file),
        name: file.name,
        file,
        sourceKind,
      })),
      status: "pending",
      outputImages: [],
    };
    setTurns((prev) => [...prev, pendingTurn]);

    try {
      const created = await createImageGeneration({
        sessionId,
        mode: draftMode,
        prompt: draftPrompt,
        resolution: draftResolution,
        referenceImages: draftImages.map((referenceImage) => referenceImage.file),
      });

      setTurns((prev) =>
        prev.map((turn) =>
          turn.id === turnId ? { ...turn, generationId: created.generationId } : turn,
        ),
      );

      setSubmissionState("streaming");
      closeStream();

      function markTurnRunning() {
        setTurns((prev) =>
          prev.map((turn) => (turn.id === turnId ? { ...turn, status: "running" } : turn)),
        );
      }

      streamCleanupRef.current = streamImageGeneration(sessionId, created.generationId, {
        onStarted: () => {
          markTurnRunning();
        },
        onRunning: () => {
          markTurnRunning();
        },
        onCompleted: (payload) => {
          const outputImages: ImageTurnOutputImage[] = payload.outputAssets
            .filter((asset) => asset.contentUrl)
            .map((asset) => {
              const raw = asset.contentUrl!;
              const contentUrl = raw.startsWith("/") ? `${apiBaseURL}${raw}` : raw;
              return {
                assetId: asset.assetId,
                contentUrl,
                mediaType: asset.mediaType,
                width: asset.width,
                height: asset.height,
                filename: asset.filename,
              };
            });
          setTurns((prev) =>
            prev.map((turn) =>
              turn.id === turnId ? { ...turn, status: "completed", outputImages } : turn,
            ),
          );
          closeStream();
          setSubmissionState("idle");
        },
        onFailed: (payload) => {
          const errorMessage = payload.errorMessage ?? "Image generation failed";
          setTurns((prev) =>
            prev.map((turn) =>
              turn.id === turnId ? { ...turn, status: "failed", errorMessage } : turn,
            ),
          );
          closeStream();
          setSubmissionState("idle");
        },
        onTransportError: (message) => {
          setTurns((prev) =>
            prev.map((turn) =>
              turn.id === turnId ? { ...turn, status: "failed", errorMessage: message } : turn,
            ),
          );
          closeStream();
          setSubmissionState("idle");
        },
      });
    } catch (error) {
      const errorMessage = toErrorMessage(error, "Failed to submit image generation");
      setTurns((prev) =>
        prev.map((turn) => (turn.id === turnId ? { ...turn, status: "failed", errorMessage } : turn)),
      );
      setSubmissionState("idle");
    }
  }

  async function handleSubmit(event: SubmitEvent<HTMLFormElement>) {
    event.preventDefault();
    await submitGeneration();
  }

  function handlePromptChange(event: ChangeEvent<HTMLTextAreaElement>) {
    setPrompt(event.target.value);
  }

  function handlePromptKeyDown(event: KeyboardEvent<HTMLTextAreaElement>) {
    if (event.nativeEvent.isComposing) {
      return;
    }

    if (event.key === "Enter" && event.shiftKey && !event.altKey && !event.ctrlKey && !event.metaKey) {
      event.preventDefault();
      void submitGeneration();
    }
  }

  function handleResolutionChange(event: ChangeEvent<HTMLSelectElement>) {
    setResolution(event.target.value);
  }

  function handleFileSelection(event: ChangeEvent<HTMLInputElement>) {
    const files = Array.from(event.target.files ?? []);
    event.target.value = "";
    if (files.length === 0) {
      return;
    }

    const newEntries = files.map((file) => createSelectedReferenceImage(file, "upload"));
    tryAddReferenceImages(newEntries);
  }

  function openFilePicker() {
    if (busy) {
      return;
    }

    fileInputRef.current?.click();
  }

  function removeReferenceImage(id: string) {
    setReferenceImages((current) => {
      const next = current.filter((referenceImage) => referenceImage.id !== id);
      referenceImagesRef.current = next;
      return next;
    });
  }

  async function reuseImage(source: ReusableImageSource) {
    if (busy) {
      return;
    }

    if (source.kind === "upload" && source.file) {
      tryAddReferenceImages([createSelectedReferenceImage(source.file, "upload")]);
      return;
    }

    if (source.kind !== "generated" || !source.contentUrl) {
      return;
    }

    setReusingImageIds((current) => [...current, source.id]);

    try {
      const response = await fetch(source.contentUrl);
      if (!response.ok) {
        throw new Error(`Request failed with ${response.status} ${response.statusText}`);
      }

      const blob = await response.blob();
      const file = new File([blob], source.name, {
        type: blob.type || source.mediaType,
      });

      tryAddReferenceImages([createSelectedReferenceImage(file, "generated")]);
    } catch (error) {
      showAttachmentToast(toErrorMessage(error, `Failed to add ${source.name} to this draft.`));
    } finally {
      setReusingImageIds((current) => current.filter((id) => id !== source.id));
    }
  }

  function closeStream() {
    streamCleanupRef.current?.();
    streamCleanupRef.current = null;
  }

  function clearAttachmentToastTimeout() {
    if (attachmentToastTimeoutRef.current === null) {
      return;
    }

    window.clearTimeout(attachmentToastTimeoutRef.current);
    attachmentToastTimeoutRef.current = null;
  }

  function clearAttachmentToast() {
    clearAttachmentToastTimeout();
    setAttachmentToast("");
  }

  function tryAddReferenceImages(newEntries: SelectedReferenceImage[]) {
    const next = [...referenceImagesRef.current, ...newEntries];
    const validationError = validateReferenceImages(
      next.map((referenceImage) => referenceImage.file),
      maxReferenceImages,
    );
    if (validationError) {
      showAttachmentToast(validationError);
      return false;
    }

    clearAttachmentToast();
    setScreenError("");
    referenceImagesRef.current = next;
    setReferenceImages(next);
    return true;
  }

  function showAttachmentToast(message: string) {
    clearAttachmentToastTimeout();
    setAttachmentToast(message);
    attachmentToastTimeoutRef.current = window.setTimeout(() => {
      attachmentToastTimeoutRef.current = null;
      setAttachmentToast("");
    }, imageToastDurationMs);
  }

  return {
    attachmentToast,
    bootstrapState,
    busy,
    canSubmit,
    composerRef,
    fileInputRef,
    generateButtonLabel,
    handleFileSelection,
    handlePromptChange,
    handlePromptKeyDown,
    handleResolutionChange,
    handleSubmit,
    mode,
    openFilePicker,
    prompt,
    referenceImages,
    reusableImages,
    reusingImageIds,
    removeReferenceImage,
    resolution,
    reuseImage,
    screenError,
    sessionId,
    submissionState,
    turns,
    workspaceSubtitle,
  };
}

function createSelectedReferenceImage(
  file: File,
  sourceKind: SelectedReferenceImage["sourceKind"],
): SelectedReferenceImage {
  return {
    id: createLocalId("ref"),
    file,
    sourceKind,
  };
}
