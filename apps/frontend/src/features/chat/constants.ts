export const allowedAttachmentTypes = new Set([
  "application/pdf",
  "image/gif",
  "image/jpeg",
  "image/png",
  "image/webp",
]);

export const attachmentInputAccept = "image/png,image/jpeg,image/webp,image/gif,application/pdf";
export const maxAttachmentBytes = 20 * 1024 * 1024;
export const maxAttachmentCount = 5;
export const attachmentToastDurationMs = 3000;
export const composerPlaceholder =
  "Ask about a screenshot, summarize a PDF, or start a plain text chat. Shift + Enter to send.";
