import {
  startTransition,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ChangeEvent,
  type FormEvent,
  type KeyboardEvent,
  type UIEvent,
} from "react";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";

import { createMessage, createSession, streamRun } from "./lib/api";

const allowedAttachmentTypes = new Set([
  "application/pdf",
  "image/gif",
  "image/jpeg",
  "image/png",
  "image/webp",
]);

const maxAttachmentBytes = 20 * 1024 * 1024;
const maxAttachmentCount = 5;

type MessageRole = "user" | "assistant";
type MessageStatus = "pending" | "completed" | "failed";
type BootstrapState = "idle" | "loading" | "ready" | "error";
type SubmissionState = "idle" | "submitting" | "streaming";

type ChatAttachment = {
  filename: string;
  mediaType: string;
  sizeBytes: number;
};

type ChatMessage = {
  id: string;
  role: MessageRole;
  status: MessageStatus;
  createdAt: string;
  text: string;
  attachments: ChatAttachment[];
  modelName?: string;
  error?: string;
};

export default function App() {
  const [bootstrapState, setBootstrapState] = useState<BootstrapState>("idle");
  const [submissionState, setSubmissionState] = useState<SubmissionState>("idle");
  const [sessionId, setSessionId] = useState("");
  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const [composerText, setComposerText] = useState("");
  const [selectedFiles, setSelectedFiles] = useState<File[]>([]);
  const [screenError, setScreenError] = useState("");
  const messagesEndRef = useRef<HTMLDivElement | null>(null);
  const streamCleanupRef = useRef<(() => void) | null>(null);
  const streamAutoScrollEnabledRef = useRef(true);

  const busy = bootstrapState === "loading" || submissionState !== "idle";
  const canSubmit = (composerText.trim() !== "" || selectedFiles.length > 0) && !busy && bootstrapState === "ready";

  const attachmentSummary = useMemo(
    () =>
      selectedFiles.map((file) => ({
        filename: file.name,
        mediaType: file.type,
        sizeBytes: file.size,
      })),
    [selectedFiles],
  );

  useEffect(() => {
    void bootstrapSession();

    return () => {
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

  async function bootstrapSession() {
    closeStream();
    streamAutoScrollEnabledRef.current = true;
    setBootstrapState("loading");
    setSubmissionState("idle");
    setScreenError("");
    setMessages([]);
    setSessionId("");
    setComposerText("");
    setSelectedFiles([]);

    try {
      const session = await createSession();
      startTransition(() => {
        setSessionId(session.sessionId);
        setBootstrapState("ready");
      });
    } catch (error) {
      setBootstrapState("error");
      setScreenError(toErrorMessage(error, "Failed to create a chat session"));
    }
  }

  async function submitComposer() {
    if (!canSubmit || sessionId === "") {
      return;
    }

    const validationError = validateAttachments(selectedFiles);
    if (validationError) {
      setScreenError(validationError);
      return;
    }

    const draftText = composerText;
    const draftFiles = selectedFiles;
    const optimisticUserId = createLocalId("user");
    const optimisticAssistantId = createLocalId("assistant");
    const now = new Date().toISOString();

    streamAutoScrollEnabledRef.current = true;
    setScreenError("");
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
        attachments: draftFiles.map(fileToAttachment),
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
        attachments: draftFiles,
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
                ? { ...message, modelName: payload.modelName ?? message.modelName, status: "pending" }
                : message,
            ),
          );
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
          setMessages((current) =>
            current.map((message) =>
              message.id === payload.messageId
                ? {
                    ...message,
                    status: "completed",
                    modelName: payload.modelName ?? message.modelName,
                  }
                : message,
            ),
          );
        },
        onRunFailed: (payload) => {
          closeStream();
          setSubmissionState("idle");
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
      setComposerText(draftText);
      setSelectedFiles(draftFiles);
      setMessages((current) =>
        current.filter((message) => message.id !== optimisticUserId && message.id !== optimisticAssistantId),
      );
      setScreenError(toErrorMessage(error, "Failed to send message"));
    }
  }

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    await submitComposer();
  }

  function handleComposerKeyDown(event: KeyboardEvent<HTMLTextAreaElement>) {
    if (event.nativeEvent.isComposing) {
      return;
    }

    if (event.key === "Enter" && event.shiftKey && !event.altKey && !event.ctrlKey && !event.metaKey) {
      event.preventDefault();
      void submitComposer();
    }
  }

  function handleFileSelection(event: ChangeEvent<HTMLInputElement>) {
    const files = Array.from(event.target.files ?? []);
    event.target.value = "";
    if (files.length === 0) {
      return;
    }

    const nextFiles = [...selectedFiles, ...files];
    const validationError = validateAttachments(nextFiles);
    if (validationError) {
      setScreenError(validationError);
      return;
    }

    setScreenError("");
    setSelectedFiles(nextFiles);
  }

  function removeAttachment(filename: string) {
    setSelectedFiles((current) => current.filter((file) => file.name !== filename));
  }

  function closeStream() {
    streamCleanupRef.current?.();
    streamCleanupRef.current = null;
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

  return (
    <div className="app-shell">
      <div className="app-backdrop app-backdrop-left" />
      <div className="app-backdrop app-backdrop-right" />

      <main className="app-frame">
        <section className="hero-panel">
          <div className="hero-intro">
            <p className="eyebrow">Anonymous Multimodal Chat</p>
            <p className="hero-copy">
              Start a disposable chat session, send text with images or PDFs, and watch the assistant stream its answer
              back in real time.
            </p>
          </div>

          <div className="hero-session">
            <div className="hero-metrics">
              <Metric label="Session" value={sessionId ? sessionId.slice(0, 8) : "Pending"} />
              <Metric label="State" value={bootstrapState} />
              <Metric label="Messages" value={String(messages.length)} />
            </div>

            <button className="secondary-button" onClick={() => void bootstrapSession()} type="button">
              New chat
            </button>
          </div>
        </section>

        <section className="chat-panel">
          <header className="chat-header">
            <p className="chat-subtitle">
              {submissionState === "streaming"
                ? "Streaming assistant output..."
                : bootstrapState === "loading"
                  ? "Creating session..."
                  : "Ready for the next turn."}
            </p>
          </header>

          <div
            className={messages.length === 0 ? "message-list message-list-empty" : "message-list"}
            onScroll={handleMessageListScroll}
          >
            {messages.length === 0 ? (
              <div className="empty-state">
                <p>No messages yet.</p>
                <span>Send a prompt to create the first turn in this temporary session.</span>
              </div>
            ) : (
              messages.map((message) => (
                <article className={`message-card message-${message.role}`} key={message.id}>
                  <div className="message-meta">
                    <span>{message.role === "user" ? "You" : "Assistant"}</span>
                    <span className={`status-badge status-${message.status}`}>{message.status}</span>
                    {message.modelName ? <span className="model-badge">{message.modelName}</span> : null}
                  </div>

                  <div className="message-body">
                    {message.text ? (
                      message.role === "assistant" ? (
                        <div className="message-markdown">
                          <ReactMarkdown remarkPlugins={[remarkGfm]} skipHtml>
                            {message.text}
                          </ReactMarkdown>
                        </div>
                      ) : (
                        <p>{message.text}</p>
                      )
                    ) : (
                      <p className="message-placeholder">Waiting for text...</p>
                    )}
                    {message.attachments.length > 0 ? (
                      <ul className="attachment-list">
                        {message.attachments.map((attachment) => (
                          <li key={`${message.id}-${attachment.filename}`}>
                            <span>{attachment.filename}</span>
                            <span>{compactMediaType(attachment.mediaType)}</span>
                            <span>{formatBytes(attachment.sizeBytes)}</span>
                          </li>
                        ))}
                      </ul>
                    ) : null}
                    {message.error ? <p className="message-error">{message.error}</p> : null}
                  </div>
                </article>
              ))
            )}
            <div ref={messagesEndRef} />
          </div>

          <form className="composer" onSubmit={handleSubmit}>
            <textarea
              id="prompt"
              className="composer-input"
              aria-label="Message"
              value={composerText}
              onChange={(event) => setComposerText(event.target.value)}
              onKeyDown={handleComposerKeyDown}
              placeholder="Ask about a screenshot, summarize a PDF, or start a plain text chat."
              rows={5}
              disabled={busy}
            />

            <div className="composer-toolbar">
              <label className="attachment-button">
                <input
                  type="file"
                  accept="image/png,image/jpeg,image/webp,image/gif,application/pdf"
                  multiple
                  onChange={handleFileSelection}
                  disabled={busy}
                />
                Add attachments
              </label>

              <div className="composer-submit">
                <span className="composer-hint">Shift + Enter to send</span>
                <button className="primary-button" disabled={!canSubmit} type="submit">
                  {submissionState === "streaming"
                    ? "Streaming..."
                    : submissionState === "submitting"
                      ? "Sending..."
                      : "Send"}
                </button>
              </div>
            </div>

            {attachmentSummary.length > 0 ? (
              <ul className="composer-attachments">
                {attachmentSummary.map((attachment) => (
                  <li key={attachment.filename}>
                    <span>{attachment.filename}</span>
                    <span>{compactMediaType(attachment.mediaType)}</span>
                    <span>{formatBytes(attachment.sizeBytes)}</span>
                    <button onClick={() => removeAttachment(attachment.filename)} type="button">
                      Remove
                    </button>
                  </li>
                ))}
              </ul>
            ) : null}

            {screenError ? <p className="screen-error">{screenError}</p> : null}
          </form>
        </section>
      </main>
    </div>
  );
}

function Metric(props: { label: string; value: string }) {
  return (
    <div className="metric-card">
      <span>{props.label}</span>
      <strong>{props.value}</strong>
    </div>
  );
}

function isScrolledToBottom(element: HTMLDivElement) {
  const scrollThreshold = 24;
  return element.scrollHeight - element.scrollTop - element.clientHeight <= scrollThreshold;
}

function validateAttachments(files: File[]) {
  if (files.length > maxAttachmentCount) {
    return `You can attach at most ${maxAttachmentCount} files at once.`;
  }

  for (const file of files) {
    if (!allowedAttachmentTypes.has(file.type)) {
      return `${file.name} uses an unsupported file type. Only images and PDFs are allowed.`;
    }
    if (file.size <= 0) {
      return `${file.name} is empty.`;
    }
    if (file.size > maxAttachmentBytes) {
      return `${file.name} exceeds the 20 MB attachment limit.`;
    }
  }

  return "";
}

function createLocalId(prefix: string) {
  return `${prefix}-${crypto.randomUUID()}`;
}

function fileToAttachment(file: File): ChatAttachment {
  return {
    filename: file.name,
    mediaType: file.type,
    sizeBytes: file.size,
  };
}

function compactMediaType(mediaType: string) {
  if (mediaType === "application/pdf") {
    return "PDF";
  }
  return mediaType.replace("image/", "").toUpperCase();
}

function formatBytes(bytes: number) {
  if (bytes < 1024) {
    return `${bytes} B`;
  }
  if (bytes < 1024 * 1024) {
    return `${(bytes / 1024).toFixed(1)} KB`;
  }
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

function toErrorMessage(error: unknown, fallback: string) {
  if (error instanceof Error && error.message.trim() !== "") {
    return error.message;
  }

  return fallback;
}
