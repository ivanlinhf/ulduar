import { act, fireEvent, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, it, vi } from "vitest";

import { imagePromptPlaceholder, imageToastDurationMs, referenceImageInputAccept } from "../constants";
import type { ImageWorkspaceTestContext } from "./imageWorkspaceTestContext";

export function registerImageWorkspaceSuite(context: ImageWorkspaceTestContext) {
  it("renders the image workspace with prompt and Generate button when New image is selected", async () => {
    context.renderApp();
    await context.waitForReady();

    await userEvent.click(screen.getByRole("button", { name: "New" }));
    await userEvent.click(screen.getByRole("menuitem", { name: "New image" }));

    await context.waitForImageReady();

    const promptInput = screen.getByLabelText("Image prompt");
    expect(promptInput).toBeInTheDocument();
    expect(promptInput).toHaveAttribute("placeholder", imagePromptPlaceholder);

    expect(screen.getByRole("button", { name: "Generate" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Send" })).not.toBeInTheDocument();
  });

  it("shows empty state in the timeline when no generations have been submitted", async () => {
    context.renderApp();
    await context.waitForReady();

    await userEvent.click(screen.getByRole("button", { name: "New" }));
    await userEvent.click(screen.getByRole("menuitem", { name: "New image" }));

    await context.waitForImageReady();

    expect(screen.getByText("No images yet.")).toBeInTheDocument();
  });

  it("shows resolution selector with backend capabilities", async () => {
    context.renderApp();
    await context.waitForReady();

    await userEvent.click(screen.getByRole("button", { name: "New" }));
    await userEvent.click(screen.getByRole("menuitem", { name: "New image" }));

    await context.waitForImageReady();

    const resolutionSelect = screen.getByRole("combobox", { name: "Size" });
    expect(resolutionSelect).toBeInTheDocument();
    expect(screen.getByRole("option", { name: "1024 × 1024" })).toBeInTheDocument();
    expect(screen.getByRole("option", { name: "1792 × 1024" })).toBeInTheDocument();
  });

  it("submits a text-to-image generation (no reference images)", async () => {
    context.mockSuccessfulCreateImageGeneration();
    const user = userEvent.setup();

    context.renderApp();
    await context.waitForReady();

    await user.click(screen.getByRole("button", { name: "New" }));
    await user.click(screen.getByRole("menuitem", { name: "New image" }));

    await context.waitForImageReady();

    await user.type(screen.getByLabelText("Image prompt"), "A sunset over the mountains");
    await user.click(screen.getByRole("button", { name: "Generate" }));

    await waitFor(() => {
      expect(context.mockedCreateImageGeneration).toHaveBeenCalledWith({
        sessionId: "22222222-2222-2222-2222-222222222222",
        mode: "text_to_image",
        prompt: "A sunset over the mountains",
        resolution: "1024x1024",
        referenceImages: [],
      });
    });
  });

  it("adds a pending turn to the timeline after submitting", async () => {
    context.mockSuccessfulCreateImageGeneration();
    const user = userEvent.setup();

    context.renderApp();
    await context.waitForReady();

    await user.click(screen.getByRole("button", { name: "New" }));
    await user.click(screen.getByRole("menuitem", { name: "New image" }));

    await context.waitForImageReady();

    await user.type(screen.getByLabelText("Image prompt"), "A sunset over the mountains");
    await user.click(screen.getByRole("button", { name: "Generate" }));

    await waitFor(() => {
      expect(screen.getByText("A sunset over the mountains")).toBeInTheDocument();
    });
  });

  it("transitions turn status through streaming and shows completed state", async () => {
    context.mockSuccessfulCreateImageGeneration();
    const user = userEvent.setup();

    context.renderApp();
    await context.waitForReady();

    await user.click(screen.getByRole("button", { name: "New" }));
    await user.click(screen.getByRole("menuitem", { name: "New image" }));

    await context.waitForImageReady();

    await user.type(screen.getByLabelText("Image prompt"), "A mountain scene");
    await user.click(screen.getByRole("button", { name: "Generate" }));

    await waitFor(() => {
      expect(screen.getByText("A mountain scene")).toBeInTheDocument();
    });

    const handlers = context.requireImageStreamHandlers();

    act(() => {
      handlers.onStarted?.({
        generationId: "gen-44444444-4444-4444-4444-444444444444",
        sessionId: "22222222-2222-2222-2222-222222222222",
        mode: "text_to_image",
        status: "running",
        prompt: "A mountain scene",
        resolution: { key: "1024x1024", width: 1024, height: 1024 },
        outputImageCount: 1,
        createdAt: "2026-03-31T10:01:00Z",
        inputAssets: [],
        outputAssets: [],
      });
    });

    await waitFor(() => {
      expect(screen.getByText("running")).toBeInTheDocument();
    });

    act(() => {
      handlers.onCompleted?.({
        generationId: "gen-44444444-4444-4444-4444-444444444444",
        sessionId: "22222222-2222-2222-2222-222222222222",
        mode: "text_to_image",
        status: "completed",
        prompt: "A mountain scene",
        resolution: { key: "1024x1024", width: 1024, height: 1024 },
        outputImageCount: 1,
        createdAt: "2026-03-31T10:01:00Z",
        completedAt: "2026-03-31T10:01:05Z",
        inputAssets: [],
        outputAssets: [
          {
            assetId: "asset-001",
            filename: "output.png",
            mediaType: "image/png",
            sizeBytes: 12345,
            sha256: "abc123",
            width: 1024,
            height: 1024,
            createdAt: "2026-03-31T10:01:05Z",
            contentUrl: "/api/v1/sessions/22222222-2222-2222-2222-222222222222/image-generations/gen-44444444-4444-4444-4444-444444444444/images/asset-001/content",
          },
        ],
      });
    });

    await waitFor(() => {
      expect(screen.getByText("completed")).toBeInTheDocument();
      const outputImg = screen.getByRole("img", { name: "output.png" });
      expect(outputImg).toBeInTheDocument();
      expect(outputImg).toHaveAttribute(
        "src",
        "http://localhost:8080/api/v1/sessions/22222222-2222-2222-2222-222222222222/image-generations/gen-44444444-4444-4444-4444-444444444444/images/asset-001/content",
      );
    });

    // Composer prompt should be cleared and ready for next turn
    expect(screen.getByLabelText("Image prompt")).toHaveValue("");
  });

  it("shows a second turn when a new generation is submitted after the first completes", async () => {
    context.mockSuccessfulCreateImageGeneration();
    const user = userEvent.setup();

    context.renderApp();
    await context.waitForReady();

    await user.click(screen.getByRole("button", { name: "New" }));
    await user.click(screen.getByRole("menuitem", { name: "New image" }));

    await context.waitForImageReady();

    // First generation
    await user.type(screen.getByLabelText("Image prompt"), "First image");
    await user.click(screen.getByRole("button", { name: "Generate" }));

    await waitFor(() => {
      expect(screen.getByText("First image")).toBeInTheDocument();
    });

    const handlers = context.requireImageStreamHandlers();
    act(() => {
      handlers.onCompleted?.({
        generationId: "gen-44444444-4444-4444-4444-444444444444",
        sessionId: "22222222-2222-2222-2222-222222222222",
        mode: "text_to_image",
        status: "completed",
        prompt: "First image",
        resolution: { key: "1024x1024", width: 1024, height: 1024 },
        outputImageCount: 1,
        createdAt: "2026-03-31T10:01:00Z",
        completedAt: "2026-03-31T10:01:05Z",
        inputAssets: [],
        outputAssets: [],
      });
    });

    await waitFor(() => {
      expect(screen.getByText("completed")).toBeInTheDocument();
    });

    // Second generation
    context.mockSuccessfulCreateImageGeneration({ generationId: "gen-55555555-5555-5555-5555-555555555555" });
    await user.type(screen.getByLabelText("Image prompt"), "Second image");
    await user.click(screen.getByRole("button", { name: "Generate" }));

    await waitFor(() => {
      expect(screen.getByText("First image")).toBeInTheDocument();
      expect(screen.getByText("Second image")).toBeInTheDocument();
    });
  });

  it("submits an image-edit generation when reference images are attached", async () => {
    context.mockSuccessfulCreateImageGeneration();
    const user = userEvent.setup();

    const { container } = context.renderApp();
    await context.waitForReady();

    await user.click(screen.getByRole("button", { name: "New" }));
    await user.click(screen.getByRole("menuitem", { name: "New image" }));

    await context.waitForImageReady();

    const referenceFile = new File(["x"], "ref.png", { type: "image/png" });
    const imageFileInput = container.querySelector(
      `input[type="file"][accept="${referenceImageInputAccept}"]`,
    ) as HTMLInputElement;
    expect(imageFileInput).not.toBeNull();
    fireEvent.change(imageFileInput, { target: { files: [referenceFile] } });

    await user.type(screen.getByLabelText("Image prompt"), "Edit this image");
    await user.click(screen.getByRole("button", { name: "Generate" }));

    await waitFor(() => {
      expect(context.mockedCreateImageGeneration).toHaveBeenCalledWith({
        sessionId: "22222222-2222-2222-2222-222222222222",
        mode: "image_edit",
        prompt: "Edit this image",
        resolution: "1024x1024",
        referenceImages: [referenceFile],
      });
    });
  });

  it("reuses a previous uploaded reference image only after the user explicitly reattaches it", async () => {
    context.mockSuccessfulCreateImageGeneration();
    const user = userEvent.setup();

    const { container } = context.renderApp();
    await context.waitForReady();

    await user.click(screen.getByRole("button", { name: "New" }));
    await user.click(screen.getByRole("menuitem", { name: "New image" }));

    await context.waitForImageReady();

    const referenceFile = new File(["first"], "earlier-ref.png", { type: "image/png" });
    const imageFileInput = container.querySelector(
      `input[type="file"][accept="${referenceImageInputAccept}"]`,
    ) as HTMLInputElement;
    expect(imageFileInput).not.toBeNull();
    fireEvent.change(imageFileInput, { target: { files: [referenceFile] } });

    await user.type(screen.getByLabelText("Image prompt"), "Use upload once");
    await user.click(screen.getByRole("button", { name: "Generate" }));

    await waitFor(() => {
      expect(context.mockedCreateImageGeneration).toHaveBeenNthCalledWith(1, {
        sessionId: "22222222-2222-2222-2222-222222222222",
        mode: "image_edit",
        prompt: "Use upload once",
        resolution: "1024x1024",
        referenceImages: [referenceFile],
      });
    });

    act(() => {
      context.requireImageStreamHandlers().onCompleted?.({
        generationId: "gen-44444444-4444-4444-4444-444444444444",
        sessionId: "22222222-2222-2222-2222-222222222222",
        mode: "image_edit",
        status: "completed",
        prompt: "Use upload once",
        resolution: { key: "1024x1024", width: 1024, height: 1024 },
        outputImageCount: 1,
        createdAt: "2026-03-31T10:01:00Z",
        completedAt: "2026-03-31T10:01:05Z",
        inputAssets: [],
        outputAssets: [],
      });
    });

    await waitFor(() => {
      expect(
        screen.getByRole("button", { name: "Attach previous upload earlier-ref.png" }),
      ).toBeInTheDocument();
    });

    await user.type(screen.getByLabelText("Image prompt"), "Do not reuse implicitly");
    await user.click(screen.getByRole("button", { name: "Generate" }));

    await waitFor(() => {
      expect(context.mockedCreateImageGeneration).toHaveBeenNthCalledWith(2, {
        sessionId: "22222222-2222-2222-2222-222222222222",
        mode: "text_to_image",
        prompt: "Do not reuse implicitly",
        resolution: "1024x1024",
        referenceImages: [],
      });
    });

    act(() => {
      context.requireImageStreamHandlers().onCompleted?.({
        generationId: "gen-44444444-4444-4444-4444-444444444444",
        sessionId: "22222222-2222-2222-2222-222222222222",
        mode: "text_to_image",
        status: "completed",
        prompt: "Do not reuse implicitly",
        resolution: { key: "1024x1024", width: 1024, height: 1024 },
        outputImageCount: 1,
        createdAt: "2026-03-31T10:02:00Z",
        completedAt: "2026-03-31T10:02:05Z",
        inputAssets: [],
        outputAssets: [],
      });
    });

    await user.click(screen.getByRole("button", { name: "Attach previous upload earlier-ref.png" }));

    await waitFor(() => {
      expect(screen.getByRole("button", { name: "Remove earlier-ref.png" })).toBeInTheDocument();
    });

    await user.type(screen.getByLabelText("Image prompt"), "Reuse the earlier upload");
    await user.click(screen.getByRole("button", { name: "Generate" }));

    await waitFor(() => {
      expect(context.mockedCreateImageGeneration).toHaveBeenNthCalledWith(3, {
        sessionId: "22222222-2222-2222-2222-222222222222",
        mode: "image_edit",
        prompt: "Reuse the earlier upload",
        resolution: "1024x1024",
        referenceImages: [referenceFile],
      });
    });
  });

  it("reuses a previous generated output by attaching it into the current draft", async () => {
    context.mockSuccessfulCreateImageGeneration();
    const user = userEvent.setup();
    const generatedImageUrl =
      "http://localhost:8080/api/v1/sessions/22222222-2222-2222-2222-222222222222/image-generations/gen-44444444-4444-4444-4444-444444444444/images/asset-001/content";

    context.mockedFetch.mockImplementation(async (input) => {
      const url =
        typeof input === "string"
          ? input
          : input instanceof URL
            ? input.toString()
            : input.url;

      if (url === generatedImageUrl) {
        return new Response(new Blob(["generated"], { type: "image/png" }), {
          status: 200,
          headers: {
            "Content-Type": "image/png",
          },
        });
      }

      return new Response(JSON.stringify({ version: "test-version" }), {
        status: 200,
        headers: {
          "Content-Type": "application/json",
        },
      });
    });

    context.renderApp();
    await context.waitForReady();

    await user.click(screen.getByRole("button", { name: "New" }));
    await user.click(screen.getByRole("menuitem", { name: "New image" }));

    await context.waitForImageReady();

    await user.type(screen.getByLabelText("Image prompt"), "Create an image to reuse");
    await user.click(screen.getByRole("button", { name: "Generate" }));

    await waitFor(() => {
      expect(context.mockedCreateImageGeneration).toHaveBeenNthCalledWith(1, {
        sessionId: "22222222-2222-2222-2222-222222222222",
        mode: "text_to_image",
        prompt: "Create an image to reuse",
        resolution: "1024x1024",
        referenceImages: [],
      });
    });

    act(() => {
      context.requireImageStreamHandlers().onCompleted?.({
        generationId: "gen-44444444-4444-4444-4444-444444444444",
        sessionId: "22222222-2222-2222-2222-222222222222",
        mode: "text_to_image",
        status: "completed",
        prompt: "Create an image to reuse",
        resolution: { key: "1024x1024", width: 1024, height: 1024 },
        outputImageCount: 1,
        createdAt: "2026-03-31T10:01:00Z",
        completedAt: "2026-03-31T10:01:05Z",
        inputAssets: [],
        outputAssets: [
          {
            assetId: "asset-001",
            filename: "generated-output.png",
            mediaType: "image/png",
            sizeBytes: 12345,
            sha256: "abc123",
            width: 1024,
            height: 1024,
            createdAt: "2026-03-31T10:01:05Z",
            contentUrl:
              "/api/v1/sessions/22222222-2222-2222-2222-222222222222/image-generations/gen-44444444-4444-4444-4444-444444444444/images/asset-001/content",
          },
        ],
      });
    });

    await waitFor(() => {
      expect(
        screen.getByRole("button", { name: "Attach generated image generated-output.png" }),
      ).toBeInTheDocument();
    });

    await user.click(screen.getByRole("button", { name: "Attach generated image generated-output.png" }));

    await waitFor(() => {
      expect(screen.getByRole("button", { name: "Remove generated-output.png" })).toBeInTheDocument();
    });

    await user.type(screen.getByLabelText("Image prompt"), "Edit with generated image");
    await user.click(screen.getByRole("button", { name: "Generate" }));

    await waitFor(() => {
      expect(context.mockedCreateImageGeneration).toHaveBeenNthCalledWith(
        2,
        expect.objectContaining({
          sessionId: "22222222-2222-2222-2222-222222222222",
          mode: "image_edit",
          prompt: "Edit with generated image",
          resolution: "1024x1024",
        }),
      );
    });

    const secondCall = context.mockedCreateImageGeneration.mock.calls[1]?.[0];
    expect(secondCall?.referenceImages).toHaveLength(1);
    expect(secondCall?.referenceImages?.[0]).toBeInstanceOf(File);
    expect(secondCall?.referenceImages?.[0]?.name).toBe("generated-output.png");
    expect(secondCall?.referenceImages?.[0]?.type).toBe("image/png");
    expect(context.mockedFetch).toHaveBeenCalledWith(generatedImageUrl);
  });

  it("disables submission during generated-image reuse and ignores stale completions after a draft reset", async () => {
    context.mockSuccessfulCreateImageGeneration();
    const user = userEvent.setup();
    const generatedImageUrl =
      "http://localhost:8080/api/v1/sessions/22222222-2222-2222-2222-222222222222/image-generations/gen-44444444-4444-4444-4444-444444444444/images/asset-001/content";
    const deferred = createDeferred<Response>();

    context.mockedFetch.mockImplementation(async (input) => {
      const url =
        typeof input === "string"
          ? input
          : input instanceof URL
            ? input.toString()
            : input.url;

      if (url === generatedImageUrl) {
        return deferred.promise;
      }

      return new Response(JSON.stringify({ version: "test-version" }), {
        status: 200,
        headers: {
          "Content-Type": "application/json",
        },
      });
    });

    context.renderApp();
    await context.waitForReady();

    await user.click(screen.getByRole("button", { name: "New" }));
    await user.click(screen.getByRole("menuitem", { name: "New image" }));

    await context.waitForImageReady();

    await user.type(screen.getByLabelText("Image prompt"), "Create an image to reuse");
    await user.click(screen.getByRole("button", { name: "Generate" }));

    await waitFor(() => {
      expect(context.mockedCreateImageGeneration).toHaveBeenCalledTimes(1);
    });

    act(() => {
      context.requireImageStreamHandlers().onCompleted?.({
        generationId: "gen-44444444-4444-4444-4444-444444444444",
        sessionId: "22222222-2222-2222-2222-222222222222",
        mode: "text_to_image",
        status: "completed",
        prompt: "Create an image to reuse",
        resolution: { key: "1024x1024", width: 1024, height: 1024 },
        outputImageCount: 1,
        createdAt: "2026-03-31T10:01:00Z",
        completedAt: "2026-03-31T10:01:05Z",
        inputAssets: [],
        outputAssets: [
          {
            assetId: "asset-001",
            filename: "generated-output.png",
            mediaType: "image/png",
            sizeBytes: 12345,
            sha256: "abc123",
            width: 1024,
            height: 1024,
            createdAt: "2026-03-31T10:01:05Z",
            contentUrl:
              "/api/v1/sessions/22222222-2222-2222-2222-222222222222/image-generations/gen-44444444-4444-4444-4444-444444444444/images/asset-001/content",
          },
        ],
      });
    });

    await waitFor(() => {
      expect(
        screen.getByRole("button", { name: "Attach generated image generated-output.png" }),
      ).toBeInTheDocument();
    });

    await user.type(screen.getByLabelText("Image prompt"), "Try to submit while reusing");
    await user.click(screen.getByRole("button", { name: "Attach generated image generated-output.png" }));

    await waitFor(() => {
      expect(screen.getByRole("button", { name: "Generate" })).toBeDisabled();
      expect(screen.getByRole("button", { name: "Add reference images" })).toBeDisabled();
      expect(context.mockedFetch).toHaveBeenCalledTimes(2);
    });

    await user.click(screen.getByRole("button", { name: "New" }));
    await user.click(screen.getByRole("menuitem", { name: "New image" }));

    await context.waitForImageReady();

    await act(async () => {
      deferred.resolve(
        new Response(new Blob(["generated"], { type: "image/png" }), {
          status: 200,
          headers: {
            "Content-Type": "image/png",
          },
        }),
      );
      await deferred.promise;
    });

    await waitFor(() => {
      expect(screen.queryByRole("button", { name: "Remove generated-output.png" })).not.toBeInTheDocument();
    });
    expect(screen.getByLabelText("Image prompt")).toHaveValue("");
  });

  it("does not start duplicate generated-image reuse fetches for the same source", async () => {
    context.mockSuccessfulCreateImageGeneration();
    const user = userEvent.setup();
    const generatedImageUrl =
      "http://localhost:8080/api/v1/sessions/22222222-2222-2222-2222-222222222222/image-generations/gen-44444444-4444-4444-4444-444444444444/images/asset-001/content";
    const deferred = createDeferred<Response>();

    context.mockedFetch.mockImplementation(async (input) => {
      const url =
        typeof input === "string"
          ? input
          : input instanceof URL
            ? input.toString()
            : input.url;

      if (url === generatedImageUrl) {
        return deferred.promise;
      }

      return new Response(JSON.stringify({ version: "test-version" }), {
        status: 200,
        headers: {
          "Content-Type": "application/json",
        },
      });
    });

    context.renderApp();
    await context.waitForReady();

    await user.click(screen.getByRole("button", { name: "New" }));
    await user.click(screen.getByRole("menuitem", { name: "New image" }));

    await context.waitForImageReady();

    await user.type(screen.getByLabelText("Image prompt"), "Create an image to reuse");
    await user.click(screen.getByRole("button", { name: "Generate" }));

    await waitFor(() => {
      expect(context.mockedCreateImageGeneration).toHaveBeenCalledTimes(1);
    });

    act(() => {
      context.requireImageStreamHandlers().onCompleted?.({
        generationId: "gen-44444444-4444-4444-4444-444444444444",
        sessionId: "22222222-2222-2222-2222-222222222222",
        mode: "text_to_image",
        status: "completed",
        prompt: "Create an image to reuse",
        resolution: { key: "1024x1024", width: 1024, height: 1024 },
        outputImageCount: 1,
        createdAt: "2026-03-31T10:01:00Z",
        completedAt: "2026-03-31T10:01:05Z",
        inputAssets: [],
        outputAssets: [
          {
            assetId: "asset-001",
            filename: "generated-output.png",
            mediaType: "image/png",
            sizeBytes: 12345,
            sha256: "abc123",
            width: 1024,
            height: 1024,
            createdAt: "2026-03-31T10:01:05Z",
            contentUrl:
              "/api/v1/sessions/22222222-2222-2222-2222-222222222222/image-generations/gen-44444444-4444-4444-4444-444444444444/images/asset-001/content",
          },
        ],
      });
    });

    const attachButton = await screen.findByRole("button", {
      name: "Attach generated image generated-output.png",
    });

    fireEvent.click(attachButton);
    fireEvent.click(attachButton);

    expect(context.mockedFetch).toHaveBeenCalledTimes(2);

    await act(async () => {
      deferred.resolve(
        new Response(new Blob(["generated"], { type: "image/png" }), {
          status: 200,
          headers: {
            "Content-Type": "image/png",
          },
        }),
      );
      await deferred.promise;
    });

    await waitFor(() => {
      expect(screen.getAllByRole("button", { name: "Remove generated-output.png" })).toHaveLength(1);
    });
  });

  it("applies the reference-image limit when reusing a prior image", async () => {
    context.mockSuccessfulCreateImageGeneration();
    const user = userEvent.setup();

    const { container } = context.renderApp();
    await context.waitForReady();

    await user.click(screen.getByRole("button", { name: "New" }));
    await user.click(screen.getByRole("menuitem", { name: "New image" }));

    await context.waitForImageReady();

    const earlierReferenceFile = new File(["earlier"], "earlier-ref.png", { type: "image/png" });
    const imageFileInput = container.querySelector(
      `input[type="file"][accept="${referenceImageInputAccept}"]`,
    ) as HTMLInputElement;
    expect(imageFileInput).not.toBeNull();
    fireEvent.change(imageFileInput, { target: { files: [earlierReferenceFile] } });

    await user.type(screen.getByLabelText("Image prompt"), "Create reusable upload");
    await user.click(screen.getByRole("button", { name: "Generate" }));

    await waitFor(() => {
      expect(context.mockedCreateImageGeneration).toHaveBeenCalledTimes(1);
    });

    act(() => {
      context.requireImageStreamHandlers().onCompleted?.({
        generationId: "gen-44444444-4444-4444-4444-444444444444",
        sessionId: "22222222-2222-2222-2222-222222222222",
        mode: "image_edit",
        status: "completed",
        prompt: "Create reusable upload",
        resolution: { key: "1024x1024", width: 1024, height: 1024 },
        outputImageCount: 1,
        createdAt: "2026-03-31T10:01:00Z",
        completedAt: "2026-03-31T10:01:05Z",
        inputAssets: [],
        outputAssets: [],
      });
    });

    const maxedOutDraftFiles = Array.from({ length: 4 }, (_, index) =>
      new File([String(index)], `draft-${index + 1}.png`, { type: "image/png" }),
    );
    fireEvent.change(imageFileInput, { target: { files: maxedOutDraftFiles } });

    await waitFor(() => {
      expect(screen.getByRole("button", { name: "Remove draft-4.png" })).toBeInTheDocument();
    });

    await user.click(screen.getByRole("button", { name: "Attach previous upload earlier-ref.png" }));

    await waitFor(() => {
      expect(screen.getByText(/at most 4 reference images/i)).toBeInTheDocument();
    });

    expect(screen.queryByRole("button", { name: "Remove earlier-ref.png" })).not.toBeInTheDocument();
  });

  it("submits on Shift+Enter", async () => {
    context.mockSuccessfulCreateImageGeneration();

    context.renderApp();
    await context.waitForReady();

    await userEvent.click(screen.getByRole("button", { name: "New" }));
    await userEvent.click(screen.getByRole("menuitem", { name: "New image" }));

    await context.waitForImageReady();

    await userEvent.type(
      screen.getByLabelText("Image prompt"),
      "Shortcut generate{Shift>}{Enter}{/Shift}",
    );

    await waitFor(() => {
      expect(context.mockedCreateImageGeneration).toHaveBeenCalledWith(
        expect.objectContaining({
          prompt: "Shortcut generate",
          mode: "text_to_image",
        }),
      );
    });
  });

  it("rejects non-image reference files and shows a toast", async () => {
    const { container } = context.renderApp();
    await context.waitForReady();

    await userEvent.click(screen.getByRole("button", { name: "New" }));
    await userEvent.click(screen.getByRole("menuitem", { name: "New image" }));

    await context.waitForImageReady();

    // Find the image workspace file input by its image-only accept attribute
    const imageFileInput = container.querySelector(
      `input[type="file"][accept="${referenceImageInputAccept}"]`,
    ) as HTMLInputElement;
    expect(imageFileInput).not.toBeNull();
    const pdfFile = new File(["x"], "doc.pdf", { type: "application/pdf" });
    fireEvent.change(imageFileInput, { target: { files: [pdfFile] } });

    await waitFor(() => {
      expect(
        screen.getByText(/unsupported file type.*Only images/i),
      ).toBeInTheDocument();
    });

    expect(context.mockedCreateImageGeneration).not.toHaveBeenCalled();
  });

  it("shows a toast when too many reference images are attached", async () => {
    const { container } = context.renderApp();
    await context.waitForReady();

    await userEvent.click(screen.getByRole("button", { name: "New" }));
    await userEvent.click(screen.getByRole("menuitem", { name: "New image" }));

    await context.waitForImageReady();

    vi.useFakeTimers();
    try {
      // Find the image workspace file input by its image-only accept attribute
      const imageFileInput = container.querySelector(
        `input[type="file"][accept="${referenceImageInputAccept}"]`,
      ) as HTMLInputElement;
      expect(imageFileInput).not.toBeNull();

      // Capabilities in test context sets maxReferenceImages to 4
      const tooManyFiles = Array.from({ length: 5 }, (_, i) =>
        new File(["x"], `img-${i + 1}.png`, { type: "image/png" }),
      );
      fireEvent.change(imageFileInput, { target: { files: tooManyFiles } });

      const toast = screen.getByText(/at most 4 reference images/i);
      expect(toast).toBeInTheDocument();
      const toastEl = toast.closest(".toast");
      expect(toastEl).toHaveClass("toast-warning");

      act(() => {
        vi.advanceTimersByTime(imageToastDurationMs);
      });

      expect(screen.queryByText(/at most 4 reference images/i)).not.toBeInTheDocument();
    } finally {
      vi.clearAllTimers();
      vi.useRealTimers();
    }
  });

  it("creates a fresh session and clears draft state when New image is selected again", async () => {
    context.mockSuccessfulCreateImageGeneration();

    context.renderApp();
    await context.waitForReady();

    await userEvent.click(screen.getByRole("button", { name: "New" }));
    await userEvent.click(screen.getByRole("menuitem", { name: "New image" }));

    await context.waitForImageReady();

    // Type a prompt
    await userEvent.type(screen.getByLabelText("Image prompt"), "Draft text");
    expect(screen.getByLabelText("Image prompt")).toHaveValue("Draft text");

    const initialImageSessionCalls = context.mockedCreateSession.mock.calls.length;

    // Click New image again
    await userEvent.click(screen.getByRole("button", { name: "New" }));
    await userEvent.click(screen.getByRole("menuitem", { name: "New image" }));

    await context.waitForImageReady();

    // A new session should have been created
    expect(context.mockedCreateSession.mock.calls.length).toBeGreaterThan(initialImageSessionCalls);

    // Prompt should be cleared
    expect(screen.getByLabelText("Image prompt")).toHaveValue("");
  });

  it("clears timeline turns when a new image session is started", async () => {
    context.mockSuccessfulCreateImageGeneration();

    context.renderApp();
    await context.waitForReady();

    await userEvent.click(screen.getByRole("button", { name: "New" }));
    await userEvent.click(screen.getByRole("menuitem", { name: "New image" }));

    await context.waitForImageReady();

    await userEvent.type(screen.getByLabelText("Image prompt"), "First turn prompt");
    await userEvent.click(screen.getByRole("button", { name: "Generate" }));

    await waitFor(() => {
      expect(screen.getByText("First turn prompt")).toBeInTheDocument();
    });

    // Start a new image session
    await userEvent.click(screen.getByRole("button", { name: "New" }));
    await userEvent.click(screen.getByRole("menuitem", { name: "New image" }));

    await context.waitForImageReady();

    // Timeline should be empty
    expect(screen.queryByText("First turn prompt")).not.toBeInTheDocument();
    expect(screen.getByText("No images yet.")).toBeInTheDocument();
  });

  it("shows a failed turn in the timeline when generation fails and prompt is not restored", async () => {
    context.renderApp();
    await context.waitForReady();

    await userEvent.click(screen.getByRole("button", { name: "New" }));
    await userEvent.click(screen.getByRole("menuitem", { name: "New image" }));

    await context.waitForImageReady();

    context.mockedCreateImageGeneration.mockRejectedValue(new Error("Server error"));

    await userEvent.type(screen.getByLabelText("Image prompt"), "A failing prompt");
    await userEvent.click(screen.getByRole("button", { name: "Generate" }));

    await waitFor(() => {
      expect(screen.getByText("Server error")).toBeInTheDocument();
      expect(screen.getByText("A failing prompt")).toBeInTheDocument();
    });

    // Prompt should NOT be restored – error is shown in the timeline turn card
    expect(screen.getByLabelText("Image prompt")).toHaveValue("");
  });

  it("shows a failed turn when stream onFailed fires", async () => {
    context.mockSuccessfulCreateImageGeneration();

    context.renderApp();
    await context.waitForReady();

    await userEvent.click(screen.getByRole("button", { name: "New" }));
    await userEvent.click(screen.getByRole("menuitem", { name: "New image" }));

    await context.waitForImageReady();

    await userEvent.type(screen.getByLabelText("Image prompt"), "A prompt that will fail");
    await userEvent.click(screen.getByRole("button", { name: "Generate" }));

    await waitFor(() => {
      expect(screen.getByText("A prompt that will fail")).toBeInTheDocument();
    });

    const handlers = context.requireImageStreamHandlers();
    act(() => {
      handlers.onFailed?.({
        generationId: "gen-44444444-4444-4444-4444-444444444444",
        sessionId: "22222222-2222-2222-2222-222222222222",
        mode: "text_to_image",
        status: "failed",
        prompt: "A prompt that will fail",
        resolution: { key: "1024x1024", width: 1024, height: 1024 },
        outputImageCount: 1,
        createdAt: "2026-03-31T10:01:00Z",
        errorMessage: "Provider returned an error",
        inputAssets: [],
        outputAssets: [],
      });
    });

    await waitFor(() => {
      expect(screen.getByText("failed")).toBeInTheDocument();
      expect(screen.getByText("Provider returned an error")).toBeInTheDocument();
    });
  });

  it("the reference image file input only accepts image types", async () => {
    const { container } = context.renderApp();
    await context.waitForReady();

    await userEvent.click(screen.getByRole("button", { name: "New" }));
    await userEvent.click(screen.getByRole("menuitem", { name: "New image" }));

    await context.waitForImageReady();

    // Find the image workspace file input by its image-only accept attribute
    const imageFileInput = container.querySelector(
      `input[type="file"][accept="${referenceImageInputAccept}"]`,
    ) as HTMLInputElement;
    expect(imageFileInput).not.toBeNull();
    expect(imageFileInput).toHaveAttribute("accept", "image/png,image/jpeg,image/webp,image/gif");
  });

  it("removes a reference image when its remove button is clicked", async () => {
    const user = userEvent.setup();
    const { container } = context.renderApp();
    await context.waitForReady();

    await user.click(screen.getByRole("button", { name: "New" }));
    await user.click(screen.getByRole("menuitem", { name: "New image" }));

    await context.waitForImageReady();

    // Find the image workspace file input by its image-only accept attribute
    const imageFileInput = container.querySelector(
      `input[type="file"][accept="${referenceImageInputAccept}"]`,
    ) as HTMLInputElement;
    expect(imageFileInput).not.toBeNull();
    const refFile = new File(["x"], "ref.png", { type: "image/png" });
    fireEvent.change(imageFileInput, { target: { files: [refFile] } });

    await waitFor(() => {
      expect(screen.getByRole("button", { name: "Remove ref.png" })).toBeInTheDocument();
    });

    await user.click(screen.getByRole("button", { name: "Remove ref.png" }));

    expect(screen.queryByRole("button", { name: "Remove ref.png" })).not.toBeInTheDocument();
  });
}

function createDeferred<T>() {
  let resolve!: (value: T | PromiseLike<T>) => void;
  let reject!: (reason?: unknown) => void;
  const promise = new Promise<T>((res, rej) => {
    resolve = res;
    reject = rej;
  });

  return { promise, reject, resolve };
}
