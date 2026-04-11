import { renderHook, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { APIError } from "./api";
import { useImageGenerationBootstrap } from "./imageGeneration";
import * as api from "./api";

vi.mock("./api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("./api")>();
  return {
    ...actual,
    getImageGenerationCapabilities: vi.fn(),
  };
});

describe("useImageGenerationBootstrap", () => {
  const mockedGetImageGenerationCapabilities = vi.mocked(api.getImageGenerationCapabilities);

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("returns feature-disabled and skips bootstrap when the feature flag is off", () => {
    const { result } = renderHook(() => useImageGenerationBootstrap(false));

    expect(result.current).toEqual({ status: "feature-disabled" });
    expect(mockedGetImageGenerationCapabilities).not.toHaveBeenCalled();
  });

  it("loads capabilities when the feature flag is on", async () => {
    const capabilities: api.ImageGenerationCapabilitiesResponse = {
      modes: ["text_to_image"],
      resolutions: [{ key: "1024x1024", width: 1024, height: 1024 }],
      maxReferenceImages: 4,
      outputImageCount: 1,
      providerName: "azure-foundry",
    };
    mockedGetImageGenerationCapabilities.mockResolvedValue(capabilities);

    const { result } = renderHook(() => useImageGenerationBootstrap(true));

    expect(result.current).toEqual({ status: "bootstrap-loading" });
    await waitFor(() => {
      expect(result.current).toEqual({ status: "available", capabilities });
    });
  });

  it("maps backend 503 responses to unavailable-503", async () => {
    mockedGetImageGenerationCapabilities.mockRejectedValue(new APIError(503, "image generation is not configured"));

    const { result } = renderHook(() => useImageGenerationBootstrap(true));

    expect(result.current).toEqual({ status: "bootstrap-loading" });
    await waitFor(() => {
      expect(result.current).toEqual({
        status: "unavailable-503",
        errorMessage: "image generation is not configured",
      });
    });
  });

  it("maps non-503 failures to generic-error", async () => {
    mockedGetImageGenerationCapabilities.mockRejectedValue(new Error("network unavailable"));

    const { result } = renderHook(() => useImageGenerationBootstrap(true));

    expect(result.current).toEqual({ status: "bootstrap-loading" });
    await waitFor(() => {
      expect(result.current).toEqual({
        status: "generic-error",
        errorMessage: "network unavailable",
      });
    });
  });
});
