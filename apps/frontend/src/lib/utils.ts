export function createLocalId(prefix: string) {
  return `${prefix}-${crypto.randomUUID()}`;
}

export function toErrorMessage(error: unknown, fallback: string) {
  if (error instanceof Error && error.message.trim() !== "") {
    return error.message;
  }

  return fallback;
}
