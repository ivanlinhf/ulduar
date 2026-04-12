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
import type { ImageBootstrapState, ImageSubmissionState, SelectedReferenceImage } from "./types";
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

  const sessionIdRef = useRef("");
  const streamCleanupRef = useRef<(() => void) | null>(null);
  const composerRef = useRef<HTMLTextAreaElement | null>(null);
  const fileInputRef = useRef<HTMLInputElement | null>(null);
  const attachmentToastTimeoutRef = useRef<number | null>(null);

  const maxReferenceImages = capabilities.maxReferenceImages;
  const mode: ImageGenerationMode = referenceImages.length > 0 ? "image_edit" : "text_to_image";
  const busy = bootstrapState === "loading" || submissionState !== "idle";
  const canSubmit = prompt.trim() !== "" && !busy && bootstrapState === "ready";

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
    sessionIdRef.current = "";
    setSessionId("");

    try {
      const session = await createSession();
      startTransition(() => {
        sessionIdRef.current = session.sessionId;
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

    setScreenError("");
    setSubmissionState("submitting");
    setPrompt("");
    setReferenceImages([]);

    try {
      const created = await createImageGeneration({
        sessionId,
        mode,
        prompt: draftPrompt,
        resolution: draftResolution,
        referenceImages: draftImages.map((r) => r.file),
      });

      setSubmissionState("streaming");
      closeStream();
      streamCleanupRef.current = streamImageGeneration(sessionId, created.generationId, {
        onStarted: () => {
          // generation acknowledged
        },
        onRunning: () => {
          // generation in progress
        },
        onCompleted: () => {
          closeStream();
          setSubmissionState("idle");
        },
        onFailed: (payload) => {
          closeStream();
          setSubmissionState("idle");
          setScreenError(payload.errorMessage ?? "Image generation failed");
        },
        onTransportError: (message) => {
          closeStream();
          setSubmissionState("idle");
          setScreenError(message);
        },
      });
    } catch (error) {
      setSubmissionState("idle");
      setPrompt(draftPrompt);
      setReferenceImages(draftImages);
      setScreenError(toErrorMessage(error, "Failed to submit image generation"));
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

    const nextImages = [
      ...referenceImages,
      ...files.map((file) => ({ id: createLocalId("ref"), file })),
    ];
    const validationError = validateReferenceImages(
      nextImages.map((r) => r.file),
      maxReferenceImages,
    );
    if (validationError) {
      showAttachmentToast(validationError);
      return;
    }

    clearAttachmentToast();
    setScreenError("");
    setReferenceImages(nextImages);
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
    workspaceSubtitle,
  };
}
