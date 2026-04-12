export const allowedReferenceImageTypes = new Set([
  "image/gif",
  "image/jpeg",
  "image/png",
  "image/webp",
]);

export const referenceImageInputAccept = "image/png,image/jpeg,image/webp,image/gif";

export const maxReferenceImageBytes = 20 * 1024 * 1024;

export const imagePromptPlaceholder =
  "Describe the image you want to generate. Shift + Enter to generate.";

export const imageToastDurationMs = 3000;
