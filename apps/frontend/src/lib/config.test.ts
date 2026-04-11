import { describe, expect, it } from "vitest";

import { createFrontendConfig } from "./config";

describe("createFrontendConfig", () => {
  it("defaults image generation to disabled when the flag is unset", () => {
    expect(createFrontendConfig({})).toEqual({
      apiBaseURL: "http://localhost:8080",
      isImageGenerationEnabled: false,
    });
  });

  it("parses the image generation flag and normalizes the API base URL", () => {
    expect(
      createFrontendConfig({
        VITE_API_BASE_URL: "https://example.com/",
        VITE_IMAGE_GENERATION_ENABLED: " true ",
      }),
    ).toEqual({
      apiBaseURL: "https://example.com",
      isImageGenerationEnabled: true,
    });
  });
});
