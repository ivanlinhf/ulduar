import { apiBaseURL } from "./config";

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

export function streamRun(sessionId: string, runId: string, handlers: StreamHandlers): () => void {
  const url = `${apiBaseURL}/api/v1/sessions/${encodeURIComponent(sessionId)}/runs/${encodeURIComponent(runId)}/stream`;
  const source = new EventSource(url);
  let closed = false;
  let terminal = false;

  source.addEventListener("run.started", (event) => {
    handlers.onRunStarted?.(parsePayload(event));
  });

  source.addEventListener("tool.status", (event) => {
    handlers.onToolStatus?.(parsePayload(event));
  });

  source.addEventListener("message.delta", (event) => {
    handlers.onMessageDelta?.(parsePayload(event));
  });

  source.addEventListener("run.completed", (event) => {
    terminal = true;
    handlers.onRunCompleted?.(parsePayload(event));
    source.close();
  });

  source.addEventListener("run.failed", (event) => {
    terminal = true;
    handlers.onRunFailed?.(parsePayload(event));
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

async function requestJSON<T>(path: string, init: RequestInit): Promise<T> {
  const response = await fetch(apiBaseURL + path, init);
  if (!response.ok) {
    throw new Error(await readErrorMessage(response));
  }

  return (await response.json()) as T;
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

function parsePayload(event: Event): StreamEventPayload {
  const message = event as MessageEvent<string>;
  return JSON.parse(message.data) as StreamEventPayload;
}
