import type { ChangeEvent, KeyboardEvent, RefObject, SubmitEvent } from "react";

import type { SelectedAttachment, SubmissionState } from "../types";
import { ActionTooltip } from "./ActionTooltip";
import { ComposerFooter } from "./ComposerFooter";
import { IconExpand } from "./icons";

type ChatComposerProps = {
  busy: boolean;
  canSubmit: boolean;
  composerText: string;
  inlineComposerRef: RefObject<HTMLTextAreaElement | null>;
  onOpenExpandedComposer: () => void;
  onOpenFilePicker: () => void;
  onRemoveAttachment: (id: string) => void;
  onSubmit: (event: SubmitEvent<HTMLFormElement>) => Promise<void>;
  onTextChange: (event: ChangeEvent<HTMLTextAreaElement>) => void;
  onTextareaKeyDown: (event: KeyboardEvent<HTMLTextAreaElement>) => void;
  screenError: string;
  selectedFiles: SelectedAttachment[];
  submissionState: SubmissionState;
  submitButtonLabel: string;
};

export function ChatComposer({
  busy,
  canSubmit,
  composerText,
  inlineComposerRef,
  onOpenExpandedComposer,
  onOpenFilePicker,
  onRemoveAttachment,
  onSubmit,
  onTextChange,
  onTextareaKeyDown,
  screenError,
  selectedFiles,
  submissionState,
  submitButtonLabel,
}: ChatComposerProps) {
  return (
    <form className="composer" onSubmit={onSubmit}>
      <div className="composer-input-shell composer-input-shell-inline">
        <textarea
          id="prompt"
          ref={inlineComposerRef}
          className="composer-input"
          aria-label="Message"
          value={composerText}
          onChange={onTextChange}
          onKeyDown={onTextareaKeyDown}
          placeholder="Ask about a screenshot, summarize a PDF, or start a plain text chat."
          rows={5}
          disabled={busy}
        />
        <ActionTooltip
          align="right"
          side="below"
          wrapperClassName="composer-expand-tooltip"
          content={<span className="action-tooltip-label">Expand editor</span>}
        >
          <button
            className="composer-expand-button"
            type="button"
            onClick={onOpenExpandedComposer}
            aria-label="Expand message editor"
            disabled={busy}
          >
            <IconExpand />
          </button>
        </ActionTooltip>
        <ComposerFooter
          busy={busy}
          canSubmit={canSubmit}
          onOpenFilePicker={onOpenFilePicker}
          onRemoveAttachment={onRemoveAttachment}
          selectedFiles={selectedFiles}
          sendButtonType="submit"
          submissionState={submissionState}
          submitButtonLabel={submitButtonLabel}
        />
      </div>

      {screenError ? <p className="screen-error">{screenError}</p> : null}
    </form>
  );
}
