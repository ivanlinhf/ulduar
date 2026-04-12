import { describe, vi } from "vitest";

import { registerImageWorkspaceSuite } from "./features/imagegen/test/registerImageWorkspaceSuite";
import { setupImageWorkspaceTestContext } from "./features/imagegen/test/imageWorkspaceTestContext";

vi.mock("./lib/api", () => ({
  createSession: vi.fn(),
  getSession: vi.fn(),
  getImageGenerationCapabilities: vi.fn(),
  createMessage: vi.fn(),
  createImageGeneration: vi.fn(),
  streamRun: vi.fn(),
  streamImageGeneration: vi.fn(),
}));

vi.mock("./lib/config", () => ({
  apiBaseURL: "http://localhost:8080",
  isImageGenerationEnabled: true,
  frontendConfig: {
    apiBaseURL: "http://localhost:8080",
    isImageGenerationEnabled: true,
  },
  createFrontendConfig: vi.fn(() => ({
    apiBaseURL: "http://localhost:8080",
    isImageGenerationEnabled: true,
  })),
}));

describe("ImageWorkspace", () => {
  const context = setupImageWorkspaceTestContext();
  registerImageWorkspaceSuite(context);
});
