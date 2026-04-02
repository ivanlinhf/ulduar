export type MessageRole = "user" | "assistant";
export type MessageStatus = "pending" | "completed" | "failed";
export type BootstrapState = "idle" | "loading" | "ready" | "error";
export type SubmissionState = "idle" | "submitting" | "streaming";

export type ChatAttachment = {
  filename: string;
  mediaType: string;
  sizeBytes: number;
};

export type ChatMessage = {
  id: string;
  role: MessageRole;
  status: MessageStatus;
  createdAt: string;
  text: string;
  attachments: ChatAttachment[];
  modelName?: string;
  error?: string;
};
