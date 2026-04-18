import { formatBytes } from "../../lib/utils";
import { maxAttachmentBytes, maxAttachmentCount } from "../chat/constants";
import { compactMediaType } from "../chat/utils";

export { attachmentToastDurationMs } from "../chat/constants";
export { compactMediaType } from "../chat/utils";
export { createLocalId, formatBytes, toErrorMessage } from "../../lib/utils";

const fallbackAllowedLabel = "JPEG, PNG, WebP images and PDFs";
const presentationOutputMediaTypePPTX =
  "application/vnd.openxmlformats-officedocument.presentationml.presentation";
const attachmentsUnsupportedMessage = "Attachments are not supported for this presentation workspace.";

export function buildPresentationAttachmentAccept(inputMediaTypes: string[]) {
  return inputMediaTypes.join(",");
}

export function presentationAttachmentsSupported(inputMediaTypes: string[]) {
  return inputMediaTypes.length > 0;
}

export function validatePresentationAttachments(files: File[], inputMediaTypes: string[]) {
  if (files.length > maxAttachmentCount) {
    return `You can attach at most ${maxAttachmentCount} files at once.`;
  }

  if (!presentationAttachmentsSupported(inputMediaTypes)) {
    return files.length > 0 ? attachmentsUnsupportedMessage : "";
  }

  const allowedTypes = new Set(inputMediaTypes);

  for (const file of files) {
    if (!allowedTypes.has(file.type)) {
      return `${file.name} uses an unsupported file type. Only ${formatPresentationAllowedTypes(inputMediaTypes)} are allowed.`;
    }
    if (file.size <= 0) {
      return `${file.name} is empty.`;
    }
    if (file.size > maxAttachmentBytes) {
      return `${file.name} exceeds the ${formatBytes(maxAttachmentBytes)} attachment limit.`;
    }
  }

  return "";
}

export function formatPresentationOutputMediaType(mediaType: string) {
  if (mediaType === presentationOutputMediaTypePPTX) {
    return "PPTX";
  }

  return compactMediaType(mediaType);
}

function formatPresentationAllowedTypes(inputMediaTypes: string[]) {
  const mediaTypes = new Set(inputMediaTypes);
  if (
    mediaTypes.has("application/pdf") &&
    mediaTypes.has("image/jpeg") &&
    mediaTypes.has("image/png") &&
    mediaTypes.has("image/webp")
  ) {
    return fallbackAllowedLabel;
  }

  const labels = inputMediaTypes.map((mediaType) => compactMediaType(mediaType));
  if (labels.length === 0) {
    return attachmentsUnsupportedMessage;
  }
  if (labels.length === 1) {
    return labels[0];
  }
  if (labels.length === 2) {
    return `${labels[0]} and ${labels[1]}`;
  }

  return `${labels.slice(0, -1).join(", ")}, and ${labels.at(-1)}`;
}
