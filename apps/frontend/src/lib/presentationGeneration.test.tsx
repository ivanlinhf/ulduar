import { renderHook, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { APIError } from "./api";
import { usePresentationGenerationBootstrap } from "./presentationGeneration";
import * as api from "./api";

vi.mock("./api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("./api")>();
  return {
    ...actual,
    getPresentationGenerationCapabilities: vi.fn(),
  };
});

describe("usePresentationGenerationBootstrap", () => {
  const mockedGetPresentationGenerationCapabilities = vi.mocked(
    api.getPresentationGenerationCapabilities,
  );

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("returns feature-disabled and skips bootstrap when the feature flag is off", () => {
    const { result } = renderHook(() => usePresentationGenerationBootstrap(false));

    expect(result.current).toEqual({ status: "feature-disabled" });
    expect(mockedGetPresentationGenerationCapabilities).not.toHaveBeenCalled();
  });

  it("loads capabilities when the feature flag is on", async () => {
    const capabilities: api.PresentationGenerationCapabilitiesResponse = {
      inputMediaTypes: ["application/pdf", "image/png"],
      outputMediaType: "application/vnd.openxmlformats-officedocument.presentationml.presentation",
      providerName: "azure-openai",
      themePresets: [{ id: "general_clean", label: "General Clean", isDefault: true }],
    };
    mockedGetPresentationGenerationCapabilities.mockResolvedValue(capabilities);

    const { result } = renderHook(() => usePresentationGenerationBootstrap(true));

    expect(result.current).toEqual({ status: "bootstrap-loading" });
    await waitFor(() => {
      expect(result.current).toEqual({ status: "available", capabilities });
    });
  });

  it("maps backend 503 responses to unavailable-503", async () => {
    mockedGetPresentationGenerationCapabilities.mockRejectedValue(
      new APIError(503, "presentation generation is not configured"),
    );

    const { result } = renderHook(() => usePresentationGenerationBootstrap(true));

    expect(result.current).toEqual({ status: "bootstrap-loading" });
    await waitFor(() => {
      expect(result.current).toEqual({
        status: "unavailable-503",
        errorMessage: "presentation generation is not configured",
      });
    });
  });

  it("maps non-503 failures to generic-error", async () => {
    mockedGetPresentationGenerationCapabilities.mockRejectedValue(new Error("network unavailable"));

    const { result } = renderHook(() => usePresentationGenerationBootstrap(true));

    expect(result.current).toEqual({ status: "bootstrap-loading" });
    await waitFor(() => {
      expect(result.current).toEqual({
        status: "generic-error",
        errorMessage: "network unavailable",
      });
    });
  });
});
