import type { ImageGenerationCapabilitiesResponse } from "../../lib/api";

export type ImageSubmissionState = "idle" | "submitting" | "streaming";

export type ImageBootstrapState = "idle" | "loading" | "ready" | "error";

export type SelectedReferenceImage = {
  id: string;
  file: File;
};

export type ImageDraft = {
  prompt: string;
  resolution: string;
  referenceImages: SelectedReferenceImage[];
};

export type ImageWorkspaceCapabilities = ImageGenerationCapabilitiesResponse;
