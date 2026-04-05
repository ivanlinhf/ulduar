import { screen, within } from "@testing-library/react";
import { expect, it } from "vitest";

import type { AppTestContext } from "./testContext";

export function registerSidebarSuite(context: AppTestContext) {
  it("renders a coming-soon chat history sidebar preview", async () => {
    context.renderApp();
    await context.waitForReady();

    const sidebar = screen.getByRole("complementary", { name: "Chat history preview" });
    expect(within(sidebar).getByText("History")).toBeInTheDocument();
    expect(within(sidebar).getByText(/History will appear here soon\./)).toBeInTheDocument();
    expect(within(sidebar).getByPlaceholderText("Search history (coming soon)")).toBeDisabled();
    expect(within(sidebar).getByRole("button", { name: "Filters unavailable" })).toBeDisabled();

    const items = within(sidebar).getAllByRole("listitem");
    expect(items).toHaveLength(6);
    expect(within(sidebar).getByText("Current chat")).toBeInTheDocument();
    expect(within(sidebar).getByText("Session browsing, paging, and delete are not available yet.")).toBeInTheDocument();
  });
}
