import type { RefObject, UIEvent } from "react";

import { MessageCard } from "./MessageCard";
import type { ChatMessage } from "../types";

type MessageListProps = {
  messages: ChatMessage[];
  messagesEndRef: RefObject<HTMLDivElement | null>;
  onScroll: (event: UIEvent<HTMLDivElement>) => void;
};

export function MessageList({ messages, messagesEndRef, onScroll }: MessageListProps) {
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
      <div ref={messagesEndRef} />
    </div>
  );
}
