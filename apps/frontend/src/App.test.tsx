import { describe, vi } from "vitest";

import { registerComposerSuite } from "./features/chat/test/registerComposerSuite";
import { registerRenderingSuite } from "./features/chat/test/registerRenderingSuite";
import { registerSessionSuite } from "./features/chat/test/registerSessionSuite";
import { setupAppTestContext } from "./features/chat/test/testContext";

vi.mock("./lib/api", () => ({
  createSession: vi.fn(),
  getSession: vi.fn(),
  createMessage: vi.fn(),
  streamRun: vi.fn(),
}));

describe("App", () => {
  const context = setupAppTestContext();

  registerSessionSuite(context);
  registerRenderingSuite(context);
  registerComposerSuite(context);
});
