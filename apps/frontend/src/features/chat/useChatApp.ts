import {
  startTransition,
  useEffect,
  useRef,
  useState,
  type ChangeEvent,
  type KeyboardEvent,
  type MouseEvent,
  type SubmitEvent,
  type UIEvent,
} from "react";

import {
  createMessage,
  createSession,
  getSession,
  streamRun,
  type MessageCitationResponse,
  type MessageResponse,
} from "../../lib/api";
import type { BootstrapState, ChatCitation, ChatMessage, SelectedAttachment, SubmissionState } from "./types";
import { attachmentToastDurationMs } from "./constants";
import {
  createLocalId,
  fileToAttachment,
  focusTextareaAtEnd,
  getFocusableElements,
  isScrolledToBottom,
  toErrorMessage,
  validateAttachments,
} from "./utils";

export function useChatApp() {
  const [bootstrapState, setBootstrapState] = useState<BootstrapState>("idle");
  const [submissionState, setSubmissionState] = useState<SubmissionState>("idle");
  const [sessionId, setSessionId] = useState("");
  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const [composerText, setComposerText] = useState("");
  const [isExpandedComposerOpen, setIsExpandedComposerOpen] = useState(false);
  const [selectedFiles, setSelectedFiles] = useState<SelectedAttachment[]>([]);
  const [screenError, setScreenError] = useState("");
  const [attachmentToast, setAttachmentToast] = useState("");
  const [transientStatus, setTransientStatus] = useState("");
  const appFrameRef = useRef<HTMLElement | null>(null);
  const dialogRef = useRef<HTMLElement | null>(null);
  const inlineComposerRef = useRef<HTMLTextAreaElement | null>(null);
  const expandedComposerRef = useRef<HTMLTextAreaElement | null>(null);
  const fileInputRef = useRef<HTMLInputElement | null>(null);
  const shouldRestoreInlineFocusRef = useRef(false);
  const messagesEndRef = useRef<HTMLDivElement | null>(null);
  const streamCleanupRef = useRef<(() => void) | null>(null);
  const streamAutoScrollEnabledRef = useRef(true);
  const attachmentToastTimeoutRef = useRef<number | null>(null);
  const sessionIdRef = useRef("");

  const busy = bootstrapState === "loading" || submissionState !== "idle";
  const canSubmit =
    (composerText.trim() !== "" || selectedFiles.length > 0) &&
    !busy &&
    bootstrapState === "ready";
  const submitButtonLabel =
    submissionState === "streaming"
      ? "Streaming..."
      : submissionState === "submitting"
        ? "Sending..."
        : "Send";
  const chatSubtitle =
    submissionState === "streaming"
      ? "Streaming assistant output..."
      : bootstrapState === "loading"
        ? "Creating session..."
        : "Ready for the next turn.";

  useEffect(() => {
    void bootstrapSession();

    return () => {
      clearAttachmentToastTimeout();
      closeStream();
    };
  }, []);

  useEffect(() => {
    scrollMessagesToEnd("smooth");
  }, [messages.length]);

  useEffect(() => {
    if (submissionState !== "streaming" || !streamAutoScrollEnabledRef.current) {
      return;
    }

    scrollMessagesToEnd("auto");
  }, [messages, submissionState]);

  useEffect(() => {
    if (!isExpandedComposerOpen) {
      document.body.style.removeProperty("overflow");
      appFrameRef.current?.removeAttribute("inert");
      if (shouldRestoreInlineFocusRef.current) {
        shouldRestoreInlineFocusRef.current = false;
        focusTextareaAtEnd(inlineComposerRef.current);
      }
      return;
    }

    document.body.style.overflow = "hidden";
    appFrameRef.current?.setAttribute("inert", "");
    focusTextareaAtEnd(expandedComposerRef.current);

    return () => {
      document.body.style.removeProperty("overflow");
      appFrameRef.current?.removeAttribute("inert");
    };
  }, [isExpandedComposerOpen]);

  async function bootstrapSession() {
    closeStream();
    clearAttachmentToast();
    streamAutoScrollEnabledRef.current = true;
    setBootstrapState("loading");
    setSubmissionState("idle");
    setScreenError("");
    setMessages([]);
    setTransientStatus("");
    sessionIdRef.current = "";
    setSessionId("");
    setComposerText("");
    setIsExpandedComposerOpen(false);
    setSelectedFiles([]);

    try {
      const session = await createSession();
      startTransition(() => {
        sessionIdRef.current = session.sessionId;
        setSessionId(session.sessionId);
        setBootstrapState("ready");
      });
    } catch (error) {
      setBootstrapState("error");
      setScreenError(toErrorMessage(error, "Failed to create a chat session"));
    }
  }

  async function submitComposer(options: { closeExpandedComposer?: boolean } = {}) {
    if (!canSubmit || sessionId === "") {
      return;
    }

    const validationError = validateAttachments(selectedFiles.map((a) => a.file));
    if (validationError) {
      showAttachmentToast(validationError);
      return;
    }

    if (options.closeExpandedComposer) {
      setIsExpandedComposerOpen(false);
    }

    const draftText = composerText;
    const draftFiles = selectedFiles;
    const optimisticUserId = createLocalId("user");
    const optimisticAssistantId = createLocalId("assistant");
    const now = new Date().toISOString();

    streamAutoScrollEnabledRef.current = true;
    setScreenError("");
    setTransientStatus("");
    setSubmissionState("submitting");
    setComposerText("");
    setSelectedFiles([]);
    setMessages((current) => [
      ...current,
      {
        id: optimisticUserId,
        role: "user",
        status: "completed",
        createdAt: now,
        text: draftText.trim(),
        attachments: draftFiles.map((a) => fileToAttachment(a.file)),
      },
      {
        id: optimisticAssistantId,
        role: "assistant",
        status: "pending",
        createdAt: now,
        text: "",
        attachments: [],
      },
    ]);

    try {
      const created = await createMessage({
        sessionId,
        text: draftText,
        attachments: draftFiles.map((a) => a.file),
      });

      setMessages((current) =>
        current.map((message) => {
          if (message.id === optimisticUserId) {
            return { ...message, id: created.userMessageId, createdAt: created.createdAt };
          }
          if (message.id === optimisticAssistantId) {
            return { ...message, id: created.assistantMessageId, createdAt: created.createdAt };
          }
          return message;
        }),
      );

      setSubmissionState("streaming");
      closeStream();
      streamCleanupRef.current = streamRun(sessionId, created.runId, {
        onRunStarted: (payload) => {
          setMessages((current) =>
            current.map((message) =>
              message.id === payload.messageId
                ? {
                    ...message,
                    modelName: payload.modelName ?? message.modelName,
                    inputTokens: payload.inputTokens ?? message.inputTokens,
                    outputTokens: payload.outputTokens ?? message.outputTokens,
                    totalTokens: payload.totalTokens ?? message.totalTokens,
                    status: "pending",
                  }
                : message,
            ),
          );
        },
        onToolStatus: (payload) => {
          if (payload.toolName !== "web_search") {
            return;
          }

          if (payload.toolPhase === "searching") {
            setTransientStatus("Searching the web...");
            return;
          }

          if (payload.toolPhase === "complete") {
            setTransientStatus("");
          }
        },
        onMessageDelta: (payload) => {
          startTransition(() => {
            setMessages((current) =>
              current.map((message) =>
                message.id === payload.messageId
                  ? { ...message, text: message.text + (payload.delta ?? ""), status: "pending" }
                  : message,
              ),
            );
          });
        },
        onRunCompleted: (payload) => {
          closeStream();
          setSubmissionState("idle");
          setTransientStatus("");
          setMessages((current) =>
            current.map((message) =>
              message.id === payload.messageId
                ? {
                    ...message,
                    status: "completed",
                    modelName: payload.modelName ?? message.modelName,
                    inputTokens: payload.inputTokens ?? message.inputTokens,
                    outputTokens: payload.outputTokens ?? message.outputTokens,
                    totalTokens: payload.totalTokens ?? message.totalTokens,
                    citations: mapCitations(payload.citations) ?? message.citations,
                  }
                : message,
            ),
          );
          void syncAssistantMessageFromSession(sessionId, payload.messageId);
        },
        onRunFailed: (payload) => {
          closeStream();
          setSubmissionState("idle");
          setTransientStatus("");
          setMessages((current) =>
            current.map((message) =>
              message.id === payload.messageId
                ? {
                    ...message,
                    status: "failed",
                    error: payload.error ?? "Run failed",
                  }
                : message,
            ),
          );
        },
        onTransportError: (message) => {
          closeStream();
          setSubmissionState("idle");
          setTransientStatus("");
          setMessages((current) =>
            current.map((item) =>
              item.id === created.assistantMessageId
                ? {
                    ...item,
                    status: "failed",
                    error: message,
                  }
                : item,
            ),
          );
        },
      });
    } catch (error) {
      setSubmissionState("idle");
      setTransientStatus("");
      setComposerText(draftText);
      setSelectedFiles(draftFiles);
      setMessages((current) =>
        current.filter((message) => message.id !== optimisticUserId && message.id !== optimisticAssistantId),
      );
      setScreenError(toErrorMessage(error, "Failed to send message"));
    }
  }

  async function handleSubmit(event: SubmitEvent<HTMLFormElement>) {
    event.preventDefault();
    await submitComposer({ closeExpandedComposer: isExpandedComposerOpen });
  }

  function handleComposerKeyDown(event: KeyboardEvent<HTMLTextAreaElement>, closeExpandedComposer = false) {
    if (event.nativeEvent.isComposing) {
      return;
    }

    if (event.key === "Enter" && event.shiftKey && !event.altKey && !event.ctrlKey && !event.metaKey) {
      event.preventDefault();
      void submitComposer({ closeExpandedComposer });
    }
  }

  function openExpandedComposer() {
    if (busy) {
      return;
    }

    setIsExpandedComposerOpen(true);
  }

  function closeExpandedComposer() {
    shouldRestoreInlineFocusRef.current = true;
    setIsExpandedComposerOpen(false);
  }

  function handleExpandedDialogKeyDown(event: KeyboardEvent<HTMLElement>) {
    if (event.key === "Escape" && !event.altKey && !event.ctrlKey && !event.metaKey && !event.shiftKey) {
      event.preventDefault();
      closeExpandedComposer();
      return;
    }

    if (event.key !== "Tab" || event.altKey || event.ctrlKey || event.metaKey) {
      return;
    }

    trapExpandedDialogFocus(event);
  }

  function trapExpandedDialogFocus(event: KeyboardEvent<HTMLElement>) {
    const focusableElements = getFocusableElements(dialogRef.current);
    if (focusableElements.length === 0) {
      event.preventDefault();
      return;
    }

    const firstElement = focusableElements[0];
    const lastElement = focusableElements[focusableElements.length - 1];
    const activeElement = document.activeElement;

    if (!event.shiftKey && activeElement === lastElement) {
      event.preventDefault();
      firstElement.focus();
    }

    if (event.shiftKey && activeElement === firstElement) {
      event.preventDefault();
      lastElement.focus();
    }
  }

  function handleFileSelection(event: ChangeEvent<HTMLInputElement>) {
    const files = Array.from(event.target.files ?? []);
    event.target.value = "";
    if (files.length === 0) {
      return;
    }

    const nextFiles = [...selectedFiles, ...files.map((file) => ({ id: createLocalId("attachment"), file }))];
    const validationError = validateAttachments(nextFiles.map((a) => a.file));
    if (validationError) {
      showAttachmentToast(validationError);
      return;
    }

    clearAttachmentToast();
    setScreenError("");
    setSelectedFiles(nextFiles);
  }

  function handleComposerTextChange(event: ChangeEvent<HTMLTextAreaElement>) {
    setComposerText(event.target.value);
  }

  function handleExpandedBackdropMouseDown(event: MouseEvent<HTMLDivElement>) {
    if (event.target === event.currentTarget) {
      closeExpandedComposer();
    }
  }

  function handleInlineComposerKeyDown(event: KeyboardEvent<HTMLTextAreaElement>) {
    handleComposerKeyDown(event);
  }

  function handleExpandedComposerKeyDown(event: KeyboardEvent<HTMLTextAreaElement>) {
    handleComposerKeyDown(event, true);
  }

  function openFilePicker() {
    if (busy) {
      return;
    }

    fileInputRef.current?.click();
  }

  function removeAttachment(id: string) {
    setSelectedFiles((current) => current.filter((a) => a.id !== id));
  }

  function closeStream() {
    streamCleanupRef.current?.();
    streamCleanupRef.current = null;
  }

  async function syncAssistantMessageFromSession(currentSessionId: string, messageId: string) {
    if (sessionIdRef.current !== currentSessionId) {
      return;
    }

    try {
      const session = await getSession(currentSessionId);
      if (sessionIdRef.current !== currentSessionId) {
        return;
      }
      const persistedMessage = session.messages.find((message) => message.messageId === messageId && message.role === "assistant");
      if (!persistedMessage) {
        return;
      }

      const persistedCitations = readMessageCitations(persistedMessage);
      setMessages((current) =>
        current.map((message) =>
          message.id === messageId
            ? {
                ...message,
                citations: persistedCitations ?? message.citations,
                modelName: persistedMessage.modelName ?? message.modelName,
                inputTokens: persistedMessage.inputTokens ?? message.inputTokens,
                outputTokens: persistedMessage.outputTokens ?? message.outputTokens,
                totalTokens: persistedMessage.totalTokens ?? message.totalTokens,
              }
            : message,
        ),
      );
    } catch {
      // Preserve the current streamed message when session refresh is unavailable.
    }
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

  function handleMessageListScroll(event: UIEvent<HTMLDivElement>) {
    if (submissionState !== "streaming" || !streamAutoScrollEnabledRef.current) {
      return;
    }

    if (!isScrolledToBottom(event.currentTarget)) {
      streamAutoScrollEnabledRef.current = false;
    }
  }

  function scrollMessagesToEnd(behavior: ScrollBehavior) {
    messagesEndRef.current?.scrollIntoView({ behavior, block: "end" });
  }

  return {
    appFrameRef,
    attachmentToast,
    bootstrapState,
    busy,
    canSubmit,
    chatSubtitle,
    composerText,
    dialogRef,
    expandedComposerRef,
    fileInputRef,
    handleComposerTextChange,
    handleExpandedBackdropMouseDown,
    handleExpandedComposerKeyDown,
    handleExpandedDialogKeyDown,
    handleFileSelection,
    handleInlineComposerKeyDown,
    handleMessageListScroll,
    handleSubmit,
    inlineComposerRef,
    isExpandedComposerOpen,
    messages,
    messagesEndRef,
    openExpandedComposer,
    openFilePicker,
    removeAttachment,
    screenError,
    selectedFiles,
    sessionId,
    submissionState,
    submitButtonLabel,
    transientStatus,
    submitFromExpandedComposer: () => submitComposer({ closeExpandedComposer: true }),
    startNewChat: () => void bootstrapSession(),
  };
}

function readMessageCitations(message: MessageResponse): ChatCitation[] | undefined {
  const citations = message.content.parts.flatMap((part) => mapCitations(part.citations) ?? []);
  return citations.length > 0 ? citations : undefined;
}

function mapCitations(citations: MessageCitationResponse[] | undefined): ChatCitation[] | undefined {
  if (!citations || citations.length === 0) {
    return undefined;
  }

  const mapped = citations.flatMap((citation) => {
    const url = citation.url?.trim();
    if (!url) {
      return [];
    }

    return [
      {
        title: citation.title?.trim() || undefined,
        url,
        startIndex: citation.startIndex,
        endIndex: citation.endIndex,
      },
    ];
  });

  return mapped.length > 0 ? mapped : undefined;
}
