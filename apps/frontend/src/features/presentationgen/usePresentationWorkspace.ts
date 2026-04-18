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
  createPresentationGeneration,
  createSession,
  getPresentationGeneration,
  streamPresentationGeneration,
  type PresentationGenerationAssetResponse,
  type PresentationGenerationCapabilitiesResponse,
  type PresentationGenerationResponse,
} from "../../lib/api";
import {
  attachmentToastDurationMs,
  buildPresentationAttachmentAccept,
  createLocalId,
  presentationAttachmentsSupported,
  toErrorMessage,
  validatePresentationAttachments,
} from "./utils";
import type {
  PresentationBootstrapState,
  PresentationSubmissionState,
  PresentationTurn,
  PresentationTurnOutputAsset,
  SelectedPresentationAttachment,
} from "./types";

const maxTransportRecoveryRetries = 3;
const transportRecoveryBaseDelayMs = 1000;
const missingOutputAssetMessage = "Presentation generation completed without a downloadable output asset.";

export function usePresentationWorkspace(capabilities: PresentationGenerationCapabilitiesResponse) {
  const [bootstrapState, setBootstrapState] = useState<PresentationBootstrapState>("idle");
  const [submissionState, setSubmissionState] = useState<PresentationSubmissionState>("idle");
  const [sessionId, setSessionId] = useState("");
  const [prompt, setPrompt] = useState("");
  const [attachments, setAttachments] = useState<SelectedPresentationAttachment[]>([]);
  const [screenError, setScreenError] = useState("");
  const [attachmentToast, setAttachmentToast] = useState("");
  const [turns, setTurns] = useState<PresentationTurn[]>([]);

  const streamCleanupRef = useRef<(() => void) | null>(null);
  const composerRef = useRef<HTMLTextAreaElement | null>(null);
  const fileInputRef = useRef<HTMLInputElement | null>(null);
  const attachmentToastTimeoutRef = useRef<number | null>(null);
  const attachmentsRef = useRef<SelectedPresentationAttachment[]>([]);
  const mountedRef = useRef(true);
  const transportRecoveryRetryCountRef = useRef(0);
  const transportRecoveryTimeoutRef = useRef<number | null>(null);

  const inputAccept = useMemo(
    () => buildPresentationAttachmentAccept(capabilities.inputMediaTypes),
    [capabilities.inputMediaTypes],
  );
  const attachmentsSupported = useMemo(
    () => presentationAttachmentsSupported(capabilities.inputMediaTypes),
    [capabilities.inputMediaTypes],
  );
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
      ? "Generating presentation..."
      : bootstrapState === "loading"
        ? "Creating session..."
        : bootstrapState === "error"
          ? "Unable to create session."
          : "Ready to generate.";

  useEffect(() => {
    attachmentsRef.current = attachments;
  }, [attachments]);

  useEffect(() => {
    mountedRef.current = true;
    void bootstrapSession();

    return () => {
      mountedRef.current = false;
      if (attachmentToastTimeoutRef.current !== null) {
        window.clearTimeout(attachmentToastTimeoutRef.current);
        attachmentToastTimeoutRef.current = null;
      }
      clearTransportRecoveryTimeout();
      closeStream();
    };
    // bootstrapSession is defined in the same hook scope and only needs to run once on mount
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  async function bootstrapSession() {
    closeStream();
    clearAttachmentToast();
    resetTransportRecovery();
    setBootstrapState("loading");
    setSubmissionState("idle");
    setScreenError("");
    setPrompt("");
    setAttachments([]);
    attachmentsRef.current = [];
    setSessionId("");
    setTurns([]);

    try {
      const session = await createSession();
      if (!mountedRef.current) {
        return;
      }
      startTransition(() => {
        setSessionId(session.sessionId);
        setBootstrapState("ready");
      });
    } catch (error) {
      if (!mountedRef.current) {
        return;
      }
      setBootstrapState("error");
      setScreenError(toErrorMessage(error, "Failed to create a presentation session"));
    }
  }

  async function submitGeneration() {
    if (!canSubmit || sessionId === "") {
      return;
    }

    const validationError = validatePresentationAttachments(
      attachmentsRef.current.map((attachment) => attachment.file),
      capabilities.inputMediaTypes,
    );
    if (validationError) {
      showAttachmentToast(validationError);
      return;
    }

    const draftPrompt = prompt;
    const draftAttachments = attachmentsRef.current;
    const draftSessionId = sessionId;
    const turnId = createLocalId("turn");

    setScreenError("");
    setSubmissionState("submitting");
    setPrompt("");
    setAttachments([]);
    attachmentsRef.current = [];
    resetTransportRecovery();

    const pendingTurn: PresentationTurn = {
      id: turnId,
      generationId: "",
      prompt: draftPrompt,
      inputAttachments: draftAttachments.map(({ id, file }) => ({
        id,
        filename: file.name,
        mediaType: file.type,
      })),
      status: "pending",
    };
    setTurns((prev) => [...prev, pendingTurn]);

    try {
      const created = await createPresentationGeneration({
        sessionId: draftSessionId,
        prompt: draftPrompt,
        attachments: draftAttachments.map((attachment) => attachment.file),
      });
      if (!mountedRef.current) {
        return;
      }

      setTurns((prev) =>
        prev.map((turn) =>
          turn.id === turnId ? { ...turn, generationId: created.generationId } : turn,
        ),
      );

      setSubmissionState("streaming");
      closeStream();
      const generationId = created.generationId;

      function markTurnRunning() {
        if (!mountedRef.current) {
          return;
        }
        setTurns((prev) =>
          prev.map((turn) => (turn.id === turnId ? { ...turn, status: "running" } : turn)),
        );
      }

      openStream(turnId, draftSessionId, generationId, markTurnRunning);
    } catch (error) {
      const errorMessage = toErrorMessage(error, "Failed to submit presentation generation");
      if (!mountedRef.current) {
        return;
      }
      failTurn(turnId, errorMessage);
      setSubmissionState("idle");
    }
  }

  function openStream(
    turnId: string,
    sessionId: string,
    generationId: string,
    markTurnRunning: () => void,
  ) {
    if (!mountedRef.current) {
      return;
    }

    streamCleanupRef.current = streamPresentationGeneration(sessionId, generationId, {
      onStarted: () => {
        resetTransportRecovery();
        markTurnRunning();
      },
      onRunning: () => {
        resetTransportRecovery();
        markTurnRunning();
      },
      onCompleted: (payload) => {
        if (!mountedRef.current) {
          return;
        }
        resetTransportRecovery();
        completeTurn(turnId, payload);
        closeStream();
        setSubmissionState("idle");
      },
      onFailed: (payload) => {
        if (!mountedRef.current) {
          return;
        }
        resetTransportRecovery();
        failTurn(turnId, payload.errorMessage ?? "Presentation generation failed");
        closeStream();
        setSubmissionState("idle");
      },
      onTransportError: (message) => {
        void reconcileTransportError(turnId, sessionId, generationId, markTurnRunning, message);
      },
    });
  }

  async function reconcileTransportError(
    turnId: string,
    sessionId: string,
    generationId: string,
    markTurnRunning: () => void,
    fallbackMessage: string,
  ) {
    if (!mountedRef.current) {
      return;
    }

    try {
      const latest = await getPresentationGeneration(sessionId, generationId);
      if (!mountedRef.current) {
        return;
      }
      if (latest.status === "completed") {
        resetTransportRecovery();
        completeTurn(turnId, latest);
        closeStream();
        setSubmissionState("idle");
      } else if (latest.status === "failed") {
        resetTransportRecovery();
        failTurn(turnId, latest.errorMessage ?? fallbackMessage);
        closeStream();
        setSubmissionState("idle");
      } else {
        const nextRetryCount = transportRecoveryRetryCountRef.current + 1;
        if (nextRetryCount > maxTransportRecoveryRetries) {
          resetTransportRecovery();
          failTurn(turnId, fallbackMessage);
          closeStream();
          setSubmissionState("idle");
          return;
        }

        transportRecoveryRetryCountRef.current = nextRetryCount;
        closeStream();
        clearTransportRecoveryTimeout();
        transportRecoveryTimeoutRef.current = window.setTimeout(() => {
          transportRecoveryTimeoutRef.current = null;
          if (!mountedRef.current) {
            return;
          }
          markTurnRunning();
          openStream(turnId, sessionId, generationId, markTurnRunning);
        }, transportRecoveryBaseDelayMs * 2 ** (nextRetryCount - 1));
      }
    } catch {
      if (!mountedRef.current) {
        return;
      }
      resetTransportRecovery();
      failTurn(turnId, fallbackMessage);
      closeStream();
      setSubmissionState("idle");
    }
  }

  function completeTurn(turnId: string, payload: PresentationGenerationResponse) {
    const outputAsset = mapOutputAsset(payload);
    if (!outputAsset) {
      failTurn(turnId, missingOutputAssetMessage);
      return;
    }

    setTurns((prev) =>
      prev.map((turn) =>
        turn.id === turnId ? { ...turn, status: "completed", outputAsset, errorMessage: undefined } : turn,
      ),
    );
  }

  function failTurn(turnId: string, errorMessage: string) {
    setTurns((prev) =>
      prev.map((turn) => (turn.id === turnId ? { ...turn, status: "failed", errorMessage } : turn)),
    );
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

  function handleFileSelection(event: ChangeEvent<HTMLInputElement>) {
    const files = Array.from(event.target.files ?? []);
    event.target.value = "";
    if (files.length === 0) {
      return;
    }

    const next = [...attachmentsRef.current, ...files.map((file) => createSelectedAttachment(file))];
    const validationError = validatePresentationAttachments(
      next.map((attachment) => attachment.file),
      capabilities.inputMediaTypes,
    );
    if (validationError) {
      showAttachmentToast(validationError);
      return;
    }

    clearAttachmentToast();
    setScreenError("");
    attachmentsRef.current = next;
    setAttachments(next);
  }

  function openFilePicker() {
    if (busy || !attachmentsSupported) {
      return;
    }

    fileInputRef.current?.click();
  }

  function removeAttachment(id: string) {
    setAttachments((current) => {
      const next = current.filter((attachment) => attachment.id !== id);
      attachmentsRef.current = next;
      return next;
    });
  }

  function closeStream() {
    streamCleanupRef.current?.();
    streamCleanupRef.current = null;
  }

  function clearTransportRecoveryTimeout() {
    if (transportRecoveryTimeoutRef.current === null) {
      return;
    }

    window.clearTimeout(transportRecoveryTimeoutRef.current);
    transportRecoveryTimeoutRef.current = null;
  }

  function resetTransportRecovery() {
    clearTransportRecoveryTimeout();
    transportRecoveryRetryCountRef.current = 0;
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
    }, attachmentToastDurationMs);
  }

  return {
    attachmentToast,
    attachments,
    bootstrapState,
    busy,
    canSubmit,
    composerRef,
    fileInputRef,
    generateButtonLabel,
    handleFileSelection,
    handlePromptChange,
    handlePromptKeyDown,
    handleSubmit,
    inputAccept,
    attachmentsSupported,
    openFilePicker,
    prompt,
    removeAttachment,
    screenError,
    sessionId,
    submissionState,
    turns,
    workspaceSubtitle,
  };
}

function createSelectedAttachment(file: File): SelectedPresentationAttachment {
  return {
    id: createLocalId("attachment"),
    file,
  };
}

function mapOutputAsset(payload: PresentationGenerationResponse): PresentationTurnOutputAsset | undefined {
  // v1 produces a single downloadable PPTX output asset for each presentation generation.
  const asset = payload.outputAssets[0];
  if (!asset) {
    return undefined;
  }

  return mapPresentationOutputAsset(payload.sessionId, payload.generationId, asset);
}

function mapPresentationOutputAsset(
  sessionId: string,
  generationId: string,
  asset: PresentationGenerationAssetResponse,
): PresentationTurnOutputAsset {
  return {
    assetId: asset.assetId,
    filename: asset.filename,
    mediaType: asset.mediaType,
    sessionId,
    generationId,
  };
}
