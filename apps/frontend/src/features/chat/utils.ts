import { allowedAttachmentTypes, maxAttachmentBytes, maxAttachmentCount } from "./constants";
import type { ChatAttachment } from "./types";
export { createLocalId, toErrorMessage } from "../../lib/utils";

export function isScrolledToBottom(element: HTMLDivElement) {
  const scrollThreshold = 24;
  return element.scrollHeight - element.scrollTop - element.clientHeight <= scrollThreshold;
}

export function validateAttachments(files: File[]) {
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

export function fileToAttachment(file: File): ChatAttachment {
  return {
    filename: file.name,
    mediaType: file.type,
    sizeBytes: file.size,
  };
}

export function focusTextareaAtEnd(textarea: HTMLTextAreaElement | null) {
  if (!textarea) {
    return;
  }

  const cursorPosition = textarea.value.length;
  textarea.focus();
  textarea.setSelectionRange(cursorPosition, cursorPosition);
}

export function getFocusableElements(container: HTMLElement | null) {
  if (!container) {
    return [];
  }

  return Array.from(
    container.querySelectorAll<HTMLElement>(
      'button:not([disabled]), textarea:not([disabled]), input:not([disabled]), select:not([disabled]), a[href], [tabindex]:not([tabindex="-1"])',
    ),
  ).filter((element) => !element.hasAttribute("disabled") && element.getAttribute("aria-hidden") !== "true");
}

export function compactMediaType(mediaType: string) {
  if (mediaType === "application/pdf") {
    return "PDF";
  }

  return mediaType.replace("image/", "").toUpperCase();
}

export function formatBytes(bytes: number) {
  if (bytes < 1024) {
    return `${bytes} B`;
  }
  if (bytes < 1024 * 1024) {
    return `${(bytes / 1024).toFixed(1)} KB`;
  }

  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}
