import { useEffect, useState, type ComponentPropsWithoutRef } from "react";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";

import { compactMediaType, formatBytes } from "../utils";
import type { ChatMessage } from "../types";

type MessageCardProps = {
  message: ChatMessage;
};

type MarkdownLinkProps = { node?: unknown } & ComponentPropsWithoutRef<"a">;
type CopyState = "idle" | "copied";

export function MessageCard({ message }: MessageCardProps) {
  const [copyState, setCopyState] = useState<CopyState>("idle");
  const tokenUsageLabel = formatTokenUsage(message);
  const tokenUsageTitle =
    message.inputTokens !== undefined || message.outputTokens !== undefined || message.totalTokens !== undefined
      ? [
          message.inputTokens !== undefined ? `in ${message.inputTokens}` : "",
          message.outputTokens !== undefined ? `out ${message.outputTokens}` : "",
          message.totalTokens !== undefined ? `total ${message.totalTokens}` : "",
        ]
          .filter(Boolean)
          .join(" / ")
      : undefined;
  const canCopyMessage = Boolean(message.text) && typeof navigator.clipboard?.writeText === "function";

  useEffect(() => {
    setCopyState("idle");
  }, [message.id, message.text]);

  useEffect(() => {
    if (copyState !== "copied") {
      return undefined;
    }

    const timeoutId = window.setTimeout(() => {
      setCopyState("idle");
    }, 2000);

    return () => {
      window.clearTimeout(timeoutId);
    };
  }, [copyState]);

  async function handleCopy() {
    if (!canCopyMessage) {
      return;
    }

    try {
      await navigator.clipboard.writeText(message.text);
      setCopyState("copied");
    } catch {
      setCopyState("idle");
    }
  }

  return (
    <article className={`message-card message-${message.role}`}>
      <div className="message-meta">
        <span>{message.role === "user" ? "You" : "Assistant"}</span>
        <span className={`status-badge status-${message.status}`}>{message.status}</span>
        {message.modelName ? <span className="model-badge">{message.modelName}</span> : null}
        {tokenUsageLabel ? (
          <span className="token-badge" title={tokenUsageTitle}>
            {tokenUsageLabel}
          </span>
        ) : null}
      </div>

      <div className="message-body">
        {message.text ? (
          message.role === "assistant" ? (
            <div className="message-markdown">
              <ReactMarkdown
                remarkPlugins={[remarkGfm]}
                components={{
                  a: ({ node: _node, ...props }: MarkdownLinkProps) => (
                    <a
                      {...props}
                      target="_blank"
                      rel="noreferrer noopener"
                    />
                  ),
                }}
              >
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

      {message.role === "assistant" ? (
        <div className="message-toolbar" role="toolbar" aria-label="Assistant message actions">
          <button
            type="button"
            className="message-toolbar-button"
            aria-label={copyState === "copied" ? "Copied assistant message" : "Copy assistant message"}
            title={copyState === "copied" ? "Copied" : "Copy"}
            onClick={() => {
              void handleCopy();
            }}
            disabled={!canCopyMessage}
          >
            {copyState === "copied" ? <CheckIcon /> : <CopyIcon />}
          </button>
          {copyState === "copied" ? (
            <span className="message-toolbar-feedback" role="status">
              Copied
            </span>
          ) : null}
        </div>
      ) : null}
    </article>
  );
}

function formatTokenUsage(message: ChatMessage): string | null {
  if (message.role !== "assistant") {
    return null;
  }
  if (message.totalTokens !== undefined) {
    return `${message.totalTokens} tokens`;
  }
  if (message.inputTokens !== undefined && message.outputTokens !== undefined) {
    return `in ${message.inputTokens} / out ${message.outputTokens}`;
  }
  return null;
}

function CopyIcon() {
  return (
    <svg viewBox="0 0 24 24" aria-hidden="true">
      <path
        d="M9 9.75A2.25 2.25 0 0 1 11.25 7.5h7.5A2.25 2.25 0 0 1 21 9.75v7.5a2.25 2.25 0 0 1-2.25 2.25h-7.5A2.25 2.25 0 0 1 9 17.25z"
        fill="none"
        stroke="currentColor"
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth="1.75"
      />
      <path
        d="M15 7.5V6.75A2.25 2.25 0 0 0 12.75 4.5h-7.5A2.25 2.25 0 0 0 3 6.75v7.5a2.25 2.25 0 0 0 2.25 2.25H6"
        fill="none"
        stroke="currentColor"
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth="1.75"
      />
    </svg>
  );
}

function CheckIcon() {
  return (
    <svg viewBox="0 0 24 24" aria-hidden="true">
      <path
        d="m5.25 12.75 4.5 4.5 9-9"
        fill="none"
        stroke="currentColor"
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth="1.75"
      />
    </svg>
  );
}
