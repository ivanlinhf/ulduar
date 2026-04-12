import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { createImageGeneration } from "./api";

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
