import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { ImageReusePicker } from "./ImageReusePicker";

describe("ImageReusePicker", () => {
  it("opens the source menu with file action and disabled session action when no reusable images exist", async () => {
    const user = userEvent.setup();

    render(
      <ImageReusePicker
        busy={false}
        onOpenFilePicker={vi.fn()}
        onReuseImage={vi.fn().mockResolvedValue(undefined)}
        reusingImageIds={[]}
        reusableImages={[]}
      />,
    );

    await user.click(screen.getByRole("button", { name: "Add reference images" }));

    expect(screen.getByRole("menuitem", { name: "From File" })).toBeInTheDocument();
    expect(screen.getByRole("menuitem", { name: "From Session" })).toHaveAttribute(
      "aria-disabled",
      "true",
    );
  });

  it("opens the session picker only when reusable images exist", async () => {
    const user = userEvent.setup();

    render(
      <ImageReusePicker
        busy={false}
        onOpenFilePicker={vi.fn()}
        onReuseImage={vi.fn().mockResolvedValue(undefined)}
        reusingImageIds={[]}
        reusableImages={[
          {
            id: "image-1",
            kind: "generated",
            name: "generated-output.png",
            mediaType: "image/png",
            contentUrl: "/generated-output.png",
          },
        ]}
      />,
    );

    await user.click(screen.getByRole("button", { name: "Add reference images" }));
    await user.click(screen.getByRole("menuitem", { name: "From Session" }));

    expect(screen.getByRole("dialog", { name: "Reference images from this session" })).toBeVisible();
    expect(
      screen.getByRole("button", { name: "Attach generated image generated-output.png" }),
    ).toBeInTheDocument();
  });
});
