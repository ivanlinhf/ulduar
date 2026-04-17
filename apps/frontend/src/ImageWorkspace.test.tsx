import { describe, vi } from "vitest";

import { registerImageWorkspaceSuite } from "./features/imagegen/test/registerImageWorkspaceSuite";
import { setupImageWorkspaceTestContext } from "./features/imagegen/test/imageWorkspaceTestContext";

vi.mock("./lib/api", () => ({
  createSession: vi.fn(),
  getSession: vi.fn(),
  getImageGenerationCapabilities: vi.fn(),
  getPresentationGenerationCapabilities: vi.fn(),
  createMessage: vi.fn(),
  createImageGeneration: vi.fn(),
  streamRun: vi.fn(),
  streamImageGeneration: vi.fn(),
}));

vi.mock("./lib/config", () => ({
  apiBaseURL: "http://localhost:8080",
  isImageGenerationEnabled: true,
  isPresentationGenerationEnabled: false,
  frontendConfig: {
    apiBaseURL: "http://localhost:8080",
    isImageGenerationEnabled: true,
    isPresentationGenerationEnabled: false,
  },
  createFrontendConfig: vi.fn(() => ({
    apiBaseURL: "http://localhost:8080",
    isImageGenerationEnabled: true,
    isPresentationGenerationEnabled: false,
  })),
}));

describe("ImageWorkspace", () => {
  const context = setupImageWorkspaceTestContext();
  registerImageWorkspaceSuite(context);
});
