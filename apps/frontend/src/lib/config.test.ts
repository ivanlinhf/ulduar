import { describe, expect, it } from "vitest";

import { createFrontendConfig } from "./config";

describe("createFrontendConfig", () => {
  it("defaults image generation to disabled when the flag is unset", () => {
    expect(createFrontendConfig({})).toEqual({
      apiBaseURL: "http://localhost:8080",
      isImageGenerationEnabled: false,
      isPresentationGenerationEnabled: false,
    });
  });

  it("parses the generation flags and normalizes the API base URL", () => {
    expect(
      createFrontendConfig({
        VITE_API_BASE_URL: "https://example.com/",
        VITE_IMAGE_GENERATION_ENABLED: " true ",
        VITE_PRESENTATION_GENERATION_ENABLED: " TRUE ",
      }),
    ).toEqual({
      apiBaseURL: "https://example.com",
      isImageGenerationEnabled: true,
      isPresentationGenerationEnabled: true,
    });
  });
});
