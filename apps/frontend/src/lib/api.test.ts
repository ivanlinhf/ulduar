import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { createImageGeneration, createPresentationGeneration } from "./api";

describe("createImageGeneration", () => {
  const mockedFetch = vi.fn<typeof fetch>();

  beforeEach(() => {
    vi.stubGlobal("fetch", mockedFetch);
  });

  afterEach(() => {
    vi.unstubAllGlobals();
    vi.clearAllMocks();
  });

  it("rejects image_edit requests without reference images before making a request", async () => {
    await expect(
      createImageGeneration({
        sessionId: "11111111-1111-1111-1111-111111111111",
        mode: "image_edit",
        prompt: "edit this image",
        resolution: "1024x1024",
      }),
    ).rejects.toThrow("image_edit requires at least one reference image");

    expect(mockedFetch).not.toHaveBeenCalled();
  });

  it("rejects text_to_image requests with reference images before making a request", async () => {
    await expect(
      createImageGeneration({
        sessionId: "11111111-1111-1111-1111-111111111111",
        mode: "text_to_image",
        prompt: "draw this",
        resolution: "1024x1024",
        referenceImages: [new File(["test"], "ref.png", { type: "image/png" })],
      }),
    ).rejects.toThrow("referenceImages are only supported for image_edit");

    expect(mockedFetch).not.toHaveBeenCalled();
  });
});

describe("createPresentationGeneration", () => {
  const mockedFetch = vi.fn<typeof fetch>();

  beforeEach(() => {
    vi.stubGlobal("fetch", mockedFetch);
  });

  afterEach(() => {
    vi.unstubAllGlobals();
    vi.clearAllMocks();
  });

  it("sends themePresetId in JSON requests when provided", async () => {
    mockedFetch.mockResolvedValue(
      new Response(
        JSON.stringify({
          generationId: "gen-1",
          status: "pending",
          createdAt: "2026-04-19T15:00:00Z",
        }),
        {
          status: 202,
          headers: { "Content-Type": "application/json" },
        },
      ),
    );

    await createPresentationGeneration({
      sessionId: "11111111-1111-1111-1111-111111111111",
      prompt: "build a travel deck",
      themePresetId: "travel_editorial",
    });

    expect(mockedFetch).toHaveBeenCalledTimes(1);
    const [, init] = mockedFetch.mock.calls[0]!;
    expect(init?.body).toBe(
      JSON.stringify({ prompt: "build a travel deck", themePresetId: "travel_editorial" }),
    );
  });

  it("sends themePresetId in multipart requests when attachments are present", async () => {
    mockedFetch.mockResolvedValue(
      new Response(
        JSON.stringify({
          generationId: "gen-2",
          status: "pending",
          createdAt: "2026-04-19T15:00:00Z",
        }),
        {
          status: 202,
          headers: { "Content-Type": "application/json" },
        },
      ),
    );

    await createPresentationGeneration({
      sessionId: "11111111-1111-1111-1111-111111111111",
      prompt: "build a travel deck",
      themePresetId: "travel_editorial",
      attachments: [new File(["pdf"], "brief.pdf", { type: "application/pdf" })],
    });

    expect(mockedFetch).toHaveBeenCalledTimes(1);
    const [, init] = mockedFetch.mock.calls[0]!;
    expect(init?.body).toBeInstanceOf(FormData);

    const formData = init?.body as FormData;
    expect(formData.get("prompt")).toBe("build a travel deck");
    expect(formData.get("themePresetId")).toBe("travel_editorial");
    expect(formData.getAll("attachments")).toHaveLength(1);
  });
});
