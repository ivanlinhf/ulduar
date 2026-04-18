import { describe, expect, it } from "vitest";

import { maxAttachmentBytes, maxAttachmentCount } from "../chat/constants";
import {
  buildPresentationAttachmentAccept,
  formatBytes,
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

  it("rejects attachment lists that exceed the maximum count", () => {
    const files = Array.from({ length: maxAttachmentCount + 1 }, (_, index) =>
      new File([`slides-${index}`], `notes-${index}.pdf`, { type: "application/pdf" }),
    );

    expect(validatePresentationAttachments(files, ["application/pdf"])).toBe(
      `You can attach at most ${maxAttachmentCount} files at once.`,
    );
  });

  it("rejects empty attachments", () => {
    const file = new File([], "empty.pdf", { type: "application/pdf" });

    expect(validatePresentationAttachments([file], ["application/pdf"])).toBe("empty.pdf is empty.");
  });

  it("rejects oversized attachments", () => {
    const file = new File(["x"], "oversized.pdf", {
      type: "application/pdf",
    });
    Object.defineProperty(file, "size", {
      value: maxAttachmentBytes + 1,
    });

    expect(validatePresentationAttachments([file], ["application/pdf"])).toBe(
      `oversized.pdf exceeds the ${formatBytes(maxAttachmentBytes)} attachment limit.`,
    );
  });
});
