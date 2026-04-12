import {
  startTransition,
  useEffect,
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
import { imageToastDurationMs } from "./constants";
import type {
  ImageBootstrapState,
  ImageSubmissionState,
  ImageTurn,
  ImageTurnOutputImage,
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
  const [screenError, setScreenError] = useState("");
  const [attachmentToast, setAttachmentToast] = useState("");
  const [turns, setTurns] = useState<ImageTurn[]>([]);

  const streamCleanupRef = useRef<(() => void) | null>(null);
  const composerRef = useRef<HTMLTextAreaElement | null>(null);
  const fileInputRef = useRef<HTMLInputElement | null>(null);
  const attachmentToastTimeoutRef = useRef<number | null>(null);

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
      referenceImages.map((r) => r.file),
      maxReferenceImages,
    );
    if (validationError) {
      showAttachmentToast(validationError);
      return;
    }

    const draftPrompt = prompt;
    const draftImages = referenceImages;
    const draftResolution = resolution;
    const draftMode = mode;
    const turnId = createLocalId("turn");

    setScreenError("");
    setSubmissionState("submitting");
    setPrompt("");
    setReferenceImages([]);

    const pendingTurn: ImageTurn = {
      id: turnId,
      generationId: "",
      prompt: draftPrompt,
      mode: draftMode,
      resolution: draftResolution,
      referenceImages: draftImages,
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
        referenceImages: draftImages.map((r) => r.file),
      });

      setTurns((prev) =>
        prev.map((t) => (t.id === turnId ? { ...t, generationId: created.generationId } : t)),
      );

      setSubmissionState("streaming");
      closeStream();

      function markTurnRunning() {
        setTurns((prev) =>
          prev.map((t) => (t.id === turnId ? { ...t, status: "running" } : t)),
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
            .filter((a) => a.contentUrl)
            .map((a) => ({
              assetId: a.assetId,
              contentUrl: a.contentUrl!,
              mediaType: a.mediaType,
              width: a.width,
              height: a.height,
              filename: a.filename,
            }));
          setTurns((prev) =>
            prev.map((t) =>
              t.id === turnId ? { ...t, status: "completed", outputImages } : t,
            ),
          );
          closeStream();
          setSubmissionState("idle");
        },
        onFailed: (payload) => {
          const errorMessage = payload.errorMessage ?? "Image generation failed";
          setTurns((prev) =>
            prev.map((t) => (t.id === turnId ? { ...t, status: "failed", errorMessage } : t)),
          );
          closeStream();
          setSubmissionState("idle");
        },
        onTransportError: (message) => {
          setTurns((prev) =>
            prev.map((t) => (t.id === turnId ? { ...t, status: "failed", errorMessage: message } : t)),
          );
          closeStream();
          setSubmissionState("idle");
        },
      });
    } catch (error) {
      const errorMessage = toErrorMessage(error, "Failed to submit image generation");
      setTurns((prev) =>
        prev.map((t) => (t.id === turnId ? { ...t, status: "failed", errorMessage } : t)),
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

    // Pre-compute entries with stable IDs before entering the updater.
    const newEntries = files.map((file) => ({ id: createLocalId("ref"), file }));

    // Validate against the current rendered snapshot. File-picker events are
    // serialized (the browser only shows one picker at a time), so the
    // closed-over referenceImages value is always the latest when this runs.
    const validationError = validateReferenceImages(
      [...referenceImages, ...newEntries].map((r) => r.file),
      maxReferenceImages,
    );
    if (validationError) {
      showAttachmentToast(validationError);
      return;
    }

    clearAttachmentToast();
    setScreenError("");
    // Use functional updater for the write so sequential selections always
    // compose against the latest committed state.
    setReferenceImages((current) => [...current, ...newEntries]);
  }

  function openFilePicker() {
    if (busy) {
      return;
    }

    fileInputRef.current?.click();
  }

  function removeReferenceImage(id: string) {
    setReferenceImages((current) => current.filter((r) => r.id !== id));
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
    removeReferenceImage,
    resolution,
    screenError,
    sessionId,
    submissionState,
    turns,
    workspaceSubtitle,
  };
}
