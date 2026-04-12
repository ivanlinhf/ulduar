import { allowedReferenceImageTypes, maxReferenceImageBytes } from "./constants";

export function validateReferenceImages(files: File[], maxCount: number): string {
  if (files.length > maxCount) {
    return `You can attach at most ${maxCount} reference image${maxCount === 1 ? "" : "s"}.`;
  }

  for (const file of files) {
    if (!allowedReferenceImageTypes.has(file.type)) {
      return `${file.name} uses an unsupported file type. Only images (PNG, JPEG, WebP, GIF) are allowed as reference images.`;
    }
    if (file.size <= 0) {
      return `${file.name} is empty.`;
    }
    if (file.size > maxReferenceImageBytes) {
      return `${file.name} exceeds the 20 MB reference image limit.`;
    }
  }

  return "";
}

export function createLocalId(prefix: string) {
  return `${prefix}-${crypto.randomUUID()}`;
}

export function toErrorMessage(error: unknown, fallback: string) {
  if (error instanceof Error && error.message.trim() !== "") {
    return error.message;
  }

  return fallback;
}
