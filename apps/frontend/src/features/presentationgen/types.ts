export type PresentationSubmissionState = "idle" | "submitting" | "streaming";

export type PresentationBootstrapState = "idle" | "loading" | "ready" | "error";

export type SelectedPresentationAttachment = {
  id: string;
  file: File;
};

export type PresentationTurnStatus = "pending" | "running" | "completed" | "failed";

export type PresentationTurnInputAttachment = {
  id: string;
  filename: string;
  mediaType: string;
};

export type PresentationTurnOutputAsset = {
  assetId: string;
  filename: string;
  mediaType: string;
  sessionId: string;
  generationId: string;
};

export type PresentationTurn = {
  id: string;
  generationId: string;
  prompt: string;
  inputAttachments: PresentationTurnInputAttachment[];
  status: PresentationTurnStatus;
  outputAsset?: PresentationTurnOutputAsset;
  errorMessage?: string;
};
