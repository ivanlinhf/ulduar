import { apiBaseURL } from "./config";

export class APIError extends Error {
  readonly status: number;

  constructor(status: number, message: string) {
    super(message);
    this.name = "APIError";
    this.status = status;
  }
}

export type SessionResponse = {
  sessionId: string;
  status: string;
  createdAt: string;
  lastMessageAt: string;
};

export type AttachmentResponse = {
  attachmentId?: string;
  filename: string;
  mediaType: string;
  sizeBytes: number;
  sha256?: string;
  providerFileId?: string;
  createdAt?: string;
};

export type CreateMessageResponse = {
  runId: string;
  userMessageId: string;
  assistantMessageId: string;
  createdAt: string;
};

export type MessageCitationResponse = {
  title?: string;
  url?: string;
  startIndex?: number;
  endIndex?: number;
};

export type MessageContentPartResponse = {
  type: string;
  text?: string;
  citations?: MessageCitationResponse[];
};

export type MessageResponse = {
  messageId: string;
  role: string;
  status: string;
  modelName?: string;
  inputTokens?: number;
  outputTokens?: number;
  totalTokens?: number;
  createdAt: string;
  content: {
    parts: MessageContentPartResponse[];
  };
  attachments: AttachmentResponse[];
};

export type SessionDetailResponse = {
  sessionId: string;
  status: string;
  createdAt: string;
  lastMessageAt: string;
  messages: MessageResponse[];
};

export type ImageGenerationMode = "text_to_image" | "image_edit";

export type ImageGenerationStatus = "pending" | "running" | "completed" | "failed";

export type ImageGenerationResolutionResponse = {
  key: string;
  width: number;
  height: number;
};

export type ImageGenerationCapabilitiesResponse = {
  modes: ImageGenerationMode[];
  resolutions: ImageGenerationResolutionResponse[];
  maxReferenceImages: number;
  outputImageCount: number;
  providerName?: string;
};

export type PresentationGenerationCapabilitiesResponse = {
  inputMediaTypes: string[];
  outputMediaType: string;
  providerName?: string;
  themePresets?: PresentationThemePresetResponse[];
};

export type PresentationThemePresetResponse = {
  id: string;
  label: string;
  description?: string;
  isDefault?: boolean;
};

export type PresentationGenerationStatus = "pending" | "running" | "completed" | "failed";

export type CreateImageGenerationResponse = {
  generationId: string;
  status: ImageGenerationStatus;
  createdAt: string;
};

export type CreatePresentationGenerationResponse = {
  generationId: string;
  status: PresentationGenerationStatus;
  createdAt: string;
};

export type ImageGenerationAssetResponse = {
  assetId: string;
  filename: string;
  mediaType: string;
  sizeBytes: number;
  sha256: string;
  width?: number;
  height?: number;
  createdAt: string;
  contentUrl?: string;
};

export type ImageGenerationResponse = {
  generationId: string;
  sessionId: string;
  mode: ImageGenerationMode;
  status: ImageGenerationStatus;
  prompt: string;
  resolution: ImageGenerationResolutionResponse;
  outputImageCount: number;
  providerName?: string;
  providerModel?: string;
  errorCode?: string;
  errorMessage?: string;
  createdAt: string;
  completedAt?: string;
  inputAssets: ImageGenerationAssetResponse[];
  outputAssets: ImageGenerationAssetResponse[];
};

export type PresentationGenerationAssetResponse = {
  assetId: string;
  filename: string;
  mediaType: string;
  sizeBytes: number;
  sha256: string;
  createdAt: string;
  contentUrl?: string;
};

export type PresentationGenerationResponse = {
  generationId: string;
  sessionId: string;
  status: PresentationGenerationStatus;
  prompt: string;
  dialectJson?: unknown;
  providerName?: string;
  providerModel?: string;
  requestedThemePresetId?: string;
  resolvedThemePresetId?: string;
  errorCode?: string;
  errorMessage?: string;
  createdAt: string;
  completedAt?: string;
  inputAssets: PresentationGenerationAssetResponse[];
  outputAssets: PresentationGenerationAssetResponse[];
};

export type ImageGenerationStreamEventName =
  | "image_generation.started"
  | "image_generation.running"
  | "image_generation.completed"
  | "image_generation.failed";

export type ImageGenerationStreamEventPayload = ImageGenerationResponse;

export type StreamEventPayload = {
  runId: string;
  messageId: string;
  responseId?: string;
  modelName?: string;
  inputTokens?: number;
  outputTokens?: number;
  totalTokens?: number;
  delta?: string;
  error?: string;
  errorCode?: string;
  toolName?: string;
  toolPhase?: string;
  citations?: MessageCitationResponse[];
};

type StreamHandlers = {
  onRunStarted?: (payload: StreamEventPayload) => void;
  onToolStatus?: (payload: StreamEventPayload) => void;
  onMessageDelta?: (payload: StreamEventPayload) => void;
  onRunCompleted?: (payload: StreamEventPayload) => void;
  onRunFailed?: (payload: StreamEventPayload) => void;
  onTransportError?: (message: string) => void;
};

export type ImageGenerationStreamHandlers = {
  onStarted?: (payload: ImageGenerationStreamEventPayload) => void;
  onRunning?: (payload: ImageGenerationStreamEventPayload) => void;
  onCompleted?: (payload: ImageGenerationStreamEventPayload) => void;
  onFailed?: (payload: ImageGenerationStreamEventPayload) => void;
  onTransportError?: (message: string) => void;
};

export type PresentationGenerationStreamHandlers = {
  onStarted?: (payload: PresentationGenerationResponse) => void;
  onRunning?: (payload: PresentationGenerationResponse) => void;
  onCompleted?: (payload: PresentationGenerationResponse) => void;
  onFailed?: (payload: PresentationGenerationResponse) => void;
  onTransportError?: (message: string) => void;
};

export async function createSession(): Promise<SessionResponse> {
  return requestJSON<SessionResponse>("/api/v1/sessions", {
    method: "POST",
  });
}

export async function getSession(sessionId: string): Promise<SessionDetailResponse> {
  return requestJSON<SessionDetailResponse>(`/api/v1/sessions/${encodeURIComponent(sessionId)}`, {
    method: "GET",
  });
}

export async function getImageGenerationCapabilities(): Promise<ImageGenerationCapabilitiesResponse> {
  return requestJSON<ImageGenerationCapabilitiesResponse>("/api/v1/image-generations/capabilities", {
    method: "GET",
  });
}

export async function getPresentationGenerationCapabilities(): Promise<PresentationGenerationCapabilitiesResponse> {
  return requestJSON<PresentationGenerationCapabilitiesResponse>(
    "/api/v1/presentation-generations/capabilities",
    {
      method: "GET",
    },
  );
}

export async function createMessage(input: {
  sessionId: string;
  text: string;
  attachments: File[];
}): Promise<CreateMessageResponse> {
  const { sessionId, text, attachments } = input;

  if (attachments.length === 0) {
    return requestJSON<CreateMessageResponse>(`/api/v1/sessions/${sessionId}/messages`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify({ text }),
    });
  }

  const formData = new FormData();
  if (text.trim() !== "") {
    formData.set("text", text);
  }
  for (const file of attachments) {
    formData.append("attachments", file);
  }

  return requestJSON<CreateMessageResponse>(`/api/v1/sessions/${sessionId}/messages`, {
    method: "POST",
    body: formData,
  });
}

export async function createImageGeneration(input: {
  sessionId: string;
  mode: ImageGenerationMode;
  prompt: string;
  resolution: string;
  referenceImages?: File[];
}): Promise<CreateImageGenerationResponse> {
  const { sessionId, mode, prompt, resolution, referenceImages = [] } = input;
  const path = `/api/v1/sessions/${encodeURIComponent(sessionId)}/image-generations`;

  validateCreateImageGenerationInput(mode, referenceImages);

  if (mode === "text_to_image") {
    return requestJSON<CreateImageGenerationResponse>(path, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify({ mode, prompt, resolution }),
    });
  }

  const formData = new FormData();
  formData.set("mode", mode);
  formData.set("prompt", prompt);
  formData.set("resolution", resolution);
  for (const file of referenceImages) {
    formData.append("referenceImages", file);
  }

  return requestJSON<CreateImageGenerationResponse>(path, {
    method: "POST",
    body: formData,
  });
}

export async function createPresentationGeneration(input: {
  sessionId: string;
  prompt: string;
  attachments?: File[];
  themePresetId?: string;
}): Promise<CreatePresentationGenerationResponse> {
  const { sessionId, prompt, attachments = [], themePresetId } = input;
  const path = `/api/v1/sessions/${encodeURIComponent(sessionId)}/presentation-generations`;
  const normalizedThemePresetId = themePresetId?.trim();

  if (attachments.length === 0) {
    const body: { prompt: string; themePresetId?: string } = { prompt };
    if (normalizedThemePresetId) {
      body.themePresetId = normalizedThemePresetId;
    }
    return requestJSON<CreatePresentationGenerationResponse>(path, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify(body),
    });
  }

  const formData = new FormData();
  formData.set("prompt", prompt);
  if (normalizedThemePresetId) {
    formData.set("themePresetId", normalizedThemePresetId);
  }
  for (const file of attachments) {
    formData.append("attachments", file);
  }

  return requestJSON<CreatePresentationGenerationResponse>(path, {
    method: "POST",
    body: formData,
  });
}

export async function getImageGeneration(sessionId: string, generationId: string): Promise<ImageGenerationResponse> {
  return requestJSON<ImageGenerationResponse>(
    `/api/v1/sessions/${encodeURIComponent(sessionId)}/image-generations/${encodeURIComponent(generationId)}`,
    {
      method: "GET",
    },
  );
}

export async function getPresentationGeneration(
  sessionId: string,
  generationId: string,
): Promise<PresentationGenerationResponse> {
  return requestJSON<PresentationGenerationResponse>(
    `/api/v1/sessions/${encodeURIComponent(sessionId)}/presentation-generations/${encodeURIComponent(generationId)}`,
    {
      method: "GET",
    },
  );
}

export function streamRun(sessionId: string, runId: string, handlers: StreamHandlers): () => void {
  const url = `${apiBaseURL}/api/v1/sessions/${encodeURIComponent(sessionId)}/runs/${encodeURIComponent(runId)}/stream`;
  const source = new EventSource(url);
  let closed = false;
  let terminal = false;

  source.addEventListener("run.started", (event) => {
    handlers.onRunStarted?.(parsePayload<StreamEventPayload>(event));
  });

  source.addEventListener("tool.status", (event) => {
    handlers.onToolStatus?.(parsePayload<StreamEventPayload>(event));
  });

  source.addEventListener("message.delta", (event) => {
    handlers.onMessageDelta?.(parsePayload<StreamEventPayload>(event));
  });

  source.addEventListener("run.completed", (event) => {
    terminal = true;
    handlers.onRunCompleted?.(parsePayload<StreamEventPayload>(event));
    source.close();
  });

  source.addEventListener("run.failed", (event) => {
    terminal = true;
    handlers.onRunFailed?.(parsePayload<StreamEventPayload>(event));
    source.close();
  });

  source.onerror = () => {
    if (closed || terminal) {
      return;
    }

    terminal = true;
    handlers.onTransportError?.("Streaming connection closed before completion");
    source.close();
  };

  return () => {
    closed = true;
    source.close();
  };
}

export function streamImageGeneration(
  sessionId: string,
  generationId: string,
  handlers: ImageGenerationStreamHandlers,
): () => void {
  const url = `${apiBaseURL}/api/v1/sessions/${encodeURIComponent(sessionId)}/image-generations/${encodeURIComponent(generationId)}/stream`;
  const source = new EventSource(url);
  let closed = false;
  let terminal = false;

  source.addEventListener("image_generation.started", (event) => {
    handlers.onStarted?.(parsePayload<ImageGenerationStreamEventPayload>(event));
  });

  source.addEventListener("image_generation.running", (event) => {
    handlers.onRunning?.(parsePayload<ImageGenerationStreamEventPayload>(event));
  });

  source.addEventListener("image_generation.completed", (event) => {
    terminal = true;
    handlers.onCompleted?.(parsePayload<ImageGenerationStreamEventPayload>(event));
    source.close();
  });

  source.addEventListener("image_generation.failed", (event) => {
    terminal = true;
    handlers.onFailed?.(parsePayload<ImageGenerationStreamEventPayload>(event));
    source.close();
  });

  source.onerror = () => {
    if (closed || terminal) {
      return;
    }

    terminal = true;
    handlers.onTransportError?.("Streaming connection closed before completion");
    source.close();
  };

  return () => {
    closed = true;
    source.close();
  };
}

export function streamPresentationGeneration(
  sessionId: string,
  generationId: string,
  handlers: PresentationGenerationStreamHandlers,
): () => void {
  const url = `${apiBaseURL}/api/v1/sessions/${encodeURIComponent(sessionId)}/presentation-generations/${encodeURIComponent(generationId)}/stream`;
  const source = new EventSource(url);
  let closed = false;
  let terminal = false;

  source.addEventListener("presentation_generation.started", (event) => {
    handlers.onStarted?.(parsePayload<PresentationGenerationResponse>(event));
  });

  source.addEventListener("presentation_generation.running", (event) => {
    handlers.onRunning?.(parsePayload<PresentationGenerationResponse>(event));
  });

  source.addEventListener("presentation_generation.completed", (event) => {
    terminal = true;
    handlers.onCompleted?.(parsePayload<PresentationGenerationResponse>(event));
    source.close();
  });

  source.addEventListener("presentation_generation.failed", (event) => {
    terminal = true;
    handlers.onFailed?.(parsePayload<PresentationGenerationResponse>(event));
    source.close();
  });

  source.onerror = () => {
    if (closed || terminal) {
      return;
    }

    terminal = true;
    handlers.onTransportError?.("Streaming connection closed before completion");
    source.close();
  };

  return () => {
    closed = true;
    source.close();
  };
}

export async function downloadPresentationGenerationAsset(
  sessionId: string,
  generationId: string,
  assetId: string,
): Promise<Blob> {
  return requestBlob(
    `/api/v1/sessions/${encodeURIComponent(sessionId)}/presentation-generations/${encodeURIComponent(generationId)}/assets/${encodeURIComponent(assetId)}/content`,
    {
      method: "GET",
    },
  );
}

async function requestJSON<T>(path: string, init: RequestInit): Promise<T> {
  const response = await fetch(apiBaseURL + path, init);
  if (!response.ok) {
    throw new APIError(response.status, await readErrorMessage(response));
  }

  return (await response.json()) as T;
}

async function requestBlob(path: string, init: RequestInit): Promise<Blob> {
  const response = await fetch(apiBaseURL + path, init);
  if (!response.ok) {
    throw new APIError(response.status, await readErrorMessage(response));
  }

  return response.blob();
}

async function readErrorMessage(response: Response): Promise<string> {
  try {
    const data = (await response.json()) as { error?: string };
    if (typeof data.error === "string" && data.error.trim() !== "") {
      return data.error;
    }
  } catch {
    // Ignore parse failures and fall back to status text.
  }

  return `Request failed with ${response.status} ${response.statusText}`;
}

function parsePayload<T>(event: Event): T {
  const message = event as MessageEvent<string>;
  return JSON.parse(message.data) as T;
}

function validateCreateImageGenerationInput(mode: ImageGenerationMode, referenceImages: File[]) {
  if (mode === "image_edit" && referenceImages.length === 0) {
    throw new Error("image_edit requires at least one reference image");
  }

  if (mode === "text_to_image" && referenceImages.length > 0) {
    throw new Error("referenceImages are only supported for image_edit");
  }
}
