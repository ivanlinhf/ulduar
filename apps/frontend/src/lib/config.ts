export type FrontendConfig = {
  apiBaseURL: string;
  isImageGenerationEnabled: boolean;
  isPresentationGenerationEnabled: boolean;
};

export function createFrontendConfig(
  env: Pick<
    ImportMetaEnv,
    "VITE_API_BASE_URL" | "VITE_IMAGE_GENERATION_ENABLED" | "VITE_PRESENTATION_GENERATION_ENABLED"
  >,
): FrontendConfig {
  return {
    apiBaseURL: (env.VITE_API_BASE_URL ?? "http://localhost:8080").replace(/\/$/, ""),
    isImageGenerationEnabled: parseBooleanFlag(env.VITE_IMAGE_GENERATION_ENABLED),
    isPresentationGenerationEnabled: parseBooleanFlag(env.VITE_PRESENTATION_GENERATION_ENABLED),
  };
}

export const frontendConfig = createFrontendConfig(import.meta.env);
export const apiBaseURL = frontendConfig.apiBaseURL;
export const isImageGenerationEnabled = frontendConfig.isImageGenerationEnabled;
export const isPresentationGenerationEnabled = frontendConfig.isPresentationGenerationEnabled;

function parseBooleanFlag(value: string | undefined): boolean {
  return value?.trim().toLowerCase() === "true";
}
