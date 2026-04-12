import type { ImageGenerationCapabilitiesResponse, ImageGenerationMode } from "../../lib/api";

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

export type ImageTurnStatus = "pending" | "running" | "completed" | "failed";

export type ImageTurnOutputImage = {
  assetId: string;
  contentUrl: string;
  mediaType: string;
  width?: number;
  height?: number;
  filename: string;
};

export type ImageTurn = {
  id: string;
  generationId: string;
  prompt: string;
  mode: ImageGenerationMode;
  resolution: string;
  referenceImages: SelectedReferenceImage[];
  status: ImageTurnStatus;
  outputImages: ImageTurnOutputImage[];
  errorMessage?: string;
};
