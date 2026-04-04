import type { ComponentPropsWithoutRef } from "react";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";

import { compactMediaType, formatBytes } from "../utils";
import type { ChatMessage } from "../types";

type MessageCardProps = {
  message: ChatMessage;
};

type MarkdownLinkProps = { node?: unknown } & ComponentPropsWithoutRef<"a">;

export function MessageCard({ message }: MessageCardProps) {
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
