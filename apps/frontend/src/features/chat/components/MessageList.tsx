import type { RefObject, UIEvent } from "react";

import { MessageCard } from "./MessageCard";
import type { ChatMessage } from "../types";

type MessageListProps = {
  messages: ChatMessage[];
  messagesEndRef: RefObject<HTMLDivElement | null>;
  onScroll: (event: UIEvent<HTMLDivElement>) => void;
  transientStatus?: string;
};

export function MessageList({ messages, messagesEndRef, onScroll, transientStatus }: MessageListProps) {
  const messageListClassName = messages.length === 0 ? "message-list message-list-empty" : "message-list";

  return (
    <div className={messageListClassName} onScroll={onScroll}>
      {messages.length === 0 ? (
        <div className="empty-state">
          <p>No messages yet.</p>
          <span>Send a prompt to create the first turn in this temporary session.</span>
        </div>
      ) : (
        messages.map((message) => (
          <MessageCard key={message.id} message={message} />
        ))
      )}
      {transientStatus ? (
        <div className="message-transient-status" role="status">
          {transientStatus}
        </div>
      ) : null}
      <div ref={messagesEndRef} />
    </div>
  );
}
