import { describe, expect, it } from "vitest";
import { screen } from "@testing-library/react";
import { http, HttpResponse } from "msw";
import TodayPage from "./TodayPage";
import type { Today } from "@/api/types";
import { renderPage } from "@/test/render";
import { server } from "@/test/server";

const empty: Today = {
  followUps: [],
  interviews: [],
  stale: [],
  trackedBoards: 0,
  pendingCaptures: 0,
};

function showToday(today: Today) {
  server.use(http.get("/api/today", () => HttpResponse.json(today)));
  return renderPage(<TodayPage />, { at: "/today", pattern: "/today" });
}

describe("TodayPage", () => {
  it("TodayPage_AllBlocksEmpty_ShowsEmptyStateNotAnError", async () => {
    showToday(empty);
    expect(
      await screen.findByText(/Nothing needs your attention/),
    ).toBeInTheDocument();
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
    expect(screen.queryByRole("region")).not.toBeInTheDocument();
  });

  it("TodayPage_SomeBlocks_ShowsOnlyNonEmptyOnesWithLinks", async () => {
    showToday({
      ...empty,
      followUps: [
        {
          jobListingId: "a",
          company: "Acme",
          title: "Dev",
          status: "sent",
          followUpAt: "2026-11-01",
        },
      ],
      interviews: [
        { jobListingId: "b", company: "Globex", at: "2026-11-09T15:00" },
      ],
      pendingCaptures: 2,
    });
    expect(
      await screen.findByRole("region", { name: "Follow-up" }),
    ).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Acme — Dev" })).toHaveAttribute(
      "href",
      "/jobs/a",
    );
    expect(screen.getByRole("link", { name: "Globex" })).toHaveAttribute(
      "href",
      "/jobs/b",
    );
    expect(screen.getByText("2026-11-09 15:00")).toBeInTheDocument();
    expect(
      screen.queryByRole("region", { name: "Stalled" }),
    ).not.toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "2 captures in the Inbox" }),
    ).toHaveAttribute("href", "/inbox");
    expect(
      screen.queryByText(/Nothing needs your attention/),
    ).not.toBeInTheDocument();
  });
});
