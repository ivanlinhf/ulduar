import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import App from "./App";
import * as api from "./lib/api";
import { currentFrontendVersion } from "./lib/frontendUpdate";

vi.mock("./lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("./lib/api")>();
  return {
    ...actual,
    createSession: vi.fn(),
    getSession: vi.fn(),
    getImageGenerationCapabilities: vi.fn(),
    getPresentationGenerationCapabilities: vi.fn(),
    createMessage: vi.fn(),
    createPresentationGeneration: vi.fn(),
    getPresentationGeneration: vi.fn(),
    downloadPresentationGenerationAsset: vi.fn(),
    streamRun: vi.fn(),
    streamImageGeneration: vi.fn(),
    streamPresentationGeneration: vi.fn(),
  };
});

vi.mock("./lib/config", () => ({
  apiBaseURL: "http://localhost:8080",
  isImageGenerationEnabled: false,
  isPresentationGenerationEnabled: true,
  frontendConfig: {
    apiBaseURL: "http://localhost:8080",
    isImageGenerationEnabled: false,
    isPresentationGenerationEnabled: true,
  },
  createFrontendConfig: vi.fn(() => ({
    apiBaseURL: "http://localhost:8080",
    isImageGenerationEnabled: false,
    isPresentationGenerationEnabled: true,
  })),
}));

describe("PresentationWorkspace", () => {
  const mockedCreateSession = vi.mocked(api.createSession);
  const mockedGetSession = vi.mocked(api.getSession);
  const mockedGetImageGenerationCapabilities = vi.mocked(api.getImageGenerationCapabilities);
  const mockedGetPresentationGenerationCapabilities = vi.mocked(
    api.getPresentationGenerationCapabilities,
  );
  const mockedCreatePresentationGeneration = vi.mocked(api.createPresentationGeneration);
  const mockedGetPresentationGeneration = vi.mocked(api.getPresentationGeneration);
  const mockedDownloadPresentationGenerationAsset = vi.mocked(
    api.downloadPresentationGenerationAsset,
  );
  const mockedStreamRun = vi.mocked(api.streamRun);
  const mockedStreamPresentationGeneration = vi.mocked(api.streamPresentationGeneration);
  const mockedFetch = vi.fn<typeof fetch>();

  let presentationStreamHandlers:
    | Parameters<typeof api.streamPresentationGeneration>[2]
    | undefined;

  beforeEach(() => {
    vi.clearAllMocks();
    vi.stubGlobal("fetch", mockedFetch);
    Object.defineProperty(URL, "createObjectURL", {
      configurable: true,
      writable: true,
      value: vi.fn(() => `blob:mock-${Math.random().toString(36).slice(2)}`),
    });
    Object.defineProperty(URL, "revokeObjectURL", {
      configurable: true,
      writable: true,
      value: vi.fn(),
    });
    mockedFetch.mockReset();
    mockedFetch.mockResolvedValue(
      new Response(JSON.stringify({ version: currentFrontendVersion }), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      }),
    );
    presentationStreamHandlers = undefined;

    mockedCreateSession
      .mockResolvedValueOnce({
        sessionId: "11111111-1111-1111-1111-111111111111",
        status: "active",
        createdAt: "2026-03-31T10:00:00Z",
        lastMessageAt: "2026-03-31T10:00:00Z",
      })
      .mockResolvedValue({
        sessionId: "22222222-2222-2222-2222-222222222222",
        status: "active",
        createdAt: "2026-03-31T10:01:00Z",
        lastMessageAt: "2026-03-31T10:01:00Z",
      });
    mockedGetSession.mockResolvedValue({
      sessionId: "11111111-1111-1111-1111-111111111111",
      status: "active",
      createdAt: "2026-03-31T10:00:00Z",
      lastMessageAt: "2026-03-31T10:01:00Z",
      messages: [],
    });
    mockedGetImageGenerationCapabilities.mockResolvedValue({
      modes: ["text_to_image"],
      resolutions: [{ key: "1024x1024", width: 1024, height: 1024 }],
      maxReferenceImages: 4,
      outputImageCount: 1,
      providerName: "azure-foundry",
    });
    mockedGetPresentationGenerationCapabilities.mockResolvedValue({
      inputMediaTypes: ["image/png", "image/jpeg", "image/webp", "application/pdf"],
      outputMediaType: "application/vnd.openxmlformats-officedocument.presentationml.presentation",
      providerName: "azure-openai",
    });
    mockedGetPresentationGeneration.mockResolvedValue({
      generationId: "gen-default",
      sessionId: "22222222-2222-2222-2222-222222222222",
      status: "running",
      prompt: "Running deck",
      createdAt: "2026-03-31T10:02:00Z",
      inputAssets: [],
      outputAssets: [],
    });
    mockedDownloadPresentationGenerationAsset.mockResolvedValue(
      new Blob(["pptx"], {
        type: "application/vnd.openxmlformats-officedocument.presentationml.presentation",
      }),
    );
    mockedStreamRun.mockImplementation(() => vi.fn());
    mockedStreamPresentationGeneration.mockImplementation((_sessionId, _generationId, handlers) => {
      presentationStreamHandlers = handlers;
      return vi.fn();
    });
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("submits prompt plus optional image/pdf references and downloads the completed PPTX", async () => {
    mockedCreatePresentationGeneration.mockResolvedValue({
      generationId: "gen-44444444-4444-4444-4444-444444444444",
      status: "pending",
      createdAt: "2026-03-31T10:02:00Z",
    });
    const user = userEvent.setup();
    const anchorClickSpy = vi.spyOn(HTMLAnchorElement.prototype, "click").mockImplementation(() => {});

    try {
      const { container } = render(<App />);
      await screen.findByText("Ready for the next turn.");
      await openPresentationWorkspace(user);

      const fileInput = container.querySelector(
        'input[type="file"][accept="image/png,image/jpeg,image/webp,application/pdf"]',
      ) as HTMLInputElement;
      const imageFile = new File(["image"], "reference.png", { type: "image/png" });
      const pdfFile = new File(["pdf"], "notes.pdf", { type: "application/pdf" });
      fireEvent.change(fileInput, { target: { files: [imageFile, pdfFile] } });

      await user.type(screen.getByLabelText("Presentation prompt"), "Build a quarterly review deck");
      await user.click(screen.getByRole("button", { name: "Generate" }));

      await waitFor(() => {
        expect(mockedCreatePresentationGeneration).toHaveBeenCalledWith({
          sessionId: "22222222-2222-2222-2222-222222222222",
          prompt: "Build a quarterly review deck",
          attachments: [imageFile, pdfFile],
        });
      });

      act(() => {
        presentationStreamHandlers?.onCompleted?.({
          generationId: "gen-44444444-4444-4444-4444-444444444444",
          sessionId: "22222222-2222-2222-2222-222222222222",
          status: "completed",
          prompt: "Build a quarterly review deck",
          createdAt: "2026-03-31T10:02:00Z",
          completedAt: "2026-03-31T10:02:08Z",
          inputAssets: [],
          outputAssets: [
            {
              assetId: "asset-001",
              filename: "quarterly-review.pptx",
              mediaType: "application/vnd.openxmlformats-officedocument.presentationml.presentation",
              sizeBytes: 4096,
              sha256: "abc123",
              createdAt: "2026-03-31T10:02:08Z",
              contentUrl:
                "/api/v1/sessions/22222222-2222-2222-2222-222222222222/presentation-generations/gen-44444444-4444-4444-4444-444444444444/assets/asset-001/content",
            },
          ],
        });
      });

      await waitFor(() => {
        expect(screen.getByText("completed")).toBeInTheDocument();
        expect(screen.getByText("quarterly-review.pptx")).toBeInTheDocument();
        expect(screen.getByText("PPTX")).toBeInTheDocument();
      });

      await user.click(
        screen.getByRole("button", { name: "Download generated presentation quarterly-review.pptx" }),
      );

      await waitFor(() => {
        expect(mockedDownloadPresentationGenerationAsset).toHaveBeenCalledWith(
          "22222222-2222-2222-2222-222222222222",
          "gen-44444444-4444-4444-4444-444444444444",
          "asset-001",
        );
        expect(anchorClickSpy).toHaveBeenCalledTimes(1);
      });
    } finally {
      anchorClickSpy.mockRestore();
    }
  });

  it("shows a validation error for unsupported attachments and leaves them out of submission", async () => {
    const user = userEvent.setup();
    const { container } = render(<App />);
    await screen.findByText("Ready for the next turn.");
    await openPresentationWorkspace(user);

    const fileInput = container.querySelector(
      'input[type="file"][accept="image/png,image/jpeg,image/webp,application/pdf"]',
    ) as HTMLInputElement;
    fireEvent.change(fileInput, {
      target: {
        files: [new File(["gif"], "animated.gif", { type: "image/gif" })],
      },
    });

    expect(
      screen.getByText(
        "animated.gif uses an unsupported file type. Only JPEG, PNG, WebP images and PDFs are allowed.",
      ),
    ).toBeInTheDocument();
    expect(screen.queryByText("animated.gif · GIF")).not.toBeInTheDocument();

    await user.type(screen.getByLabelText("Presentation prompt"), "Deck with invalid reference");
    await user.click(screen.getByRole("button", { name: "Generate" }));

    await waitFor(() => {
      expect(mockedCreatePresentationGeneration).toHaveBeenCalledWith({
        sessionId: "22222222-2222-2222-2222-222222222222",
        prompt: "Deck with invalid reference",
        attachments: [],
      });
    });
  });

  it("shows failed status when the backend stream reports a failure", async () => {
    mockedCreatePresentationGeneration.mockResolvedValue({
      generationId: "gen-55555555-5555-5555-5555-555555555555",
      status: "pending",
      createdAt: "2026-03-31T10:02:00Z",
    });
    const user = userEvent.setup();

    render(<App />);
    await screen.findByText("Ready for the next turn.");
    await openPresentationWorkspace(user);

    await user.type(screen.getByLabelText("Presentation prompt"), "Deck that fails");
    await user.click(screen.getByRole("button", { name: "Generate" }));

    await waitFor(() => {
      expect(mockedCreatePresentationGeneration).toHaveBeenCalledTimes(1);
    });

    act(() => {
      presentationStreamHandlers?.onFailed?.({
        generationId: "gen-55555555-5555-5555-5555-555555555555",
        sessionId: "22222222-2222-2222-2222-222222222222",
        status: "failed",
        prompt: "Deck that fails",
        createdAt: "2026-03-31T10:02:00Z",
        errorMessage: "planner returned invalid output",
        inputAssets: [],
        outputAssets: [],
      });
    });

    await waitFor(() => {
      expect(screen.getByText("failed")).toBeInTheDocument();
      expect(screen.getByText("planner returned invalid output")).toBeInTheDocument();
    });
  });

  it(
    "backs off and stops retrying after repeated transport failures while the backend still reports running",
    async () => {
    mockedCreatePresentationGeneration.mockResolvedValue({
      generationId: "gen-66666666-6666-6666-6666-666666666666",
      status: "pending",
      createdAt: "2026-03-31T10:02:00Z",
    });

      const user = userEvent.setup();
      render(<App />);
      await screen.findByText("Ready for the next turn.");
      await openPresentationWorkspace(user);

      await user.type(screen.getByLabelText("Presentation prompt"), "Deck with unstable stream");
      await user.click(screen.getByRole("button", { name: "Generate" }));

      await waitFor(() => {
        expect(mockedStreamPresentationGeneration).toHaveBeenCalledTimes(1);
      });

      vi.useFakeTimers();

      let expectedGetCalls = 0;
      for (const delay of [1000, 2000, 4000]) {
        await act(async () => {
          presentationStreamHandlers?.onTransportError?.("Streaming connection closed before completion");
          await Promise.resolve();
        });
        expectedGetCalls += 1;

        expect(mockedGetPresentationGeneration).toHaveBeenCalledTimes(expectedGetCalls);

        await act(async () => {
          vi.advanceTimersByTime(delay);
          await Promise.resolve();
        });

        expect(mockedStreamPresentationGeneration).toHaveBeenCalledTimes(expectedGetCalls + 1);
      }

      await act(async () => {
        presentationStreamHandlers?.onTransportError?.("Streaming connection closed before completion");
        await Promise.resolve();
      });

      expect(mockedGetPresentationGeneration).toHaveBeenCalledTimes(4);
      expect(screen.getByText("failed")).toBeInTheDocument();
      expect(screen.getByText("Streaming connection closed before completion")).toBeInTheDocument();

      act(() => {
        vi.advanceTimersByTime(10000);
      });

      expect(mockedStreamPresentationGeneration).toHaveBeenCalledTimes(4);
    },
    10000,
  );

  it("does not retry stream recovery after the workspace unmounts", async () => {
    mockedCreatePresentationGeneration.mockResolvedValue({
      generationId: "gen-77777777-7777-7777-7777-777777777777",
      status: "pending",
      createdAt: "2026-03-31T10:02:00Z",
    });
    const user = userEvent.setup();
    const view = render(<App />);
    await screen.findByText("Ready for the next turn.");
    await openPresentationWorkspace(user);

    await user.type(screen.getByLabelText("Presentation prompt"), "Deck that unmounts");
    await user.click(screen.getByRole("button", { name: "Generate" }));

    await waitFor(() => {
      expect(mockedStreamPresentationGeneration).toHaveBeenCalledTimes(1);
    });

    view.unmount();

    act(() => {
      presentationStreamHandlers?.onTransportError?.("Streaming connection closed before completion");
    });

    expect(mockedGetPresentationGeneration).not.toHaveBeenCalled();
    expect(mockedStreamPresentationGeneration).toHaveBeenCalledTimes(1);
  });

  async function openPresentationWorkspace(user: ReturnType<typeof userEvent.setup>) {
    await user.click(screen.getByRole("button", { name: "New" }));
    await user.click(screen.getByRole("menuitem", { name: "New Presentation" }));
    await screen.findByText("Ready to generate.");
  }
});
