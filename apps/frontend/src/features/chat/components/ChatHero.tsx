import { IconNewChat } from "./icons";
import { Metric } from "./Metric";
import type { BootstrapState } from "../types";

type ChatHeroProps = {
  bootstrapState: BootstrapState;
  messageCount: number;
  onNewChat: () => void;
  sessionId: string;
};

export function ChatHero({ bootstrapState, messageCount, onNewChat, sessionId }: ChatHeroProps) {
  return (
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
          <Metric label="Messages" value={String(messageCount)} />
        </div>

        <button
          aria-label="New chat"
          className="secondary-button icon-only-button new-chat-button"
          onClick={onNewChat}
          title="New chat"
          type="button"
        >
          <IconNewChat />
        </button>
      </div>
    </section>
  );
}
