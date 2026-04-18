import { describe, expect, it } from "vitest";

import {
  buildPresentationAttachmentAccept,
  presentationAttachmentsSupported,
  validatePresentationAttachments,
} from "./utils";

describe("presentation attachment utilities", () => {
  it("treats empty input media types as attachments unsupported", () => {
    expect(presentationAttachmentsSupported([])).toBe(false);
    expect(buildPresentationAttachmentAccept([])).toBe("");

    const file = new File(["slides"], "notes.pdf", { type: "application/pdf" });
    expect(validatePresentationAttachments([file], [])).toBe(
      "Attachments are not supported for this presentation workspace.",
    );
  });

  it("accepts backend-advertised media types", () => {
    const file = new File(["slides"], "notes.pdf", { type: "application/pdf" });

    expect(
      validatePresentationAttachments([file], ["application/pdf", "image/png"]),
    ).toBe("");
  });
});
