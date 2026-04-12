import { formatBytes } from "../../lib/utils";
import { allowedReferenceImageTypes, maxReferenceImageBytes } from "./constants";

export { createLocalId, toErrorMessage } from "../../lib/utils";

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
      return `${file.name} exceeds the ${formatBytes(maxReferenceImageBytes)} reference image limit.`;
    }
  }

  return "";
}
