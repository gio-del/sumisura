import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { getToday } from "@/api/client";
import type { Today, TodayApplication } from "@/api/types";

function heading(item: { company: string; title?: string }): string {
  return item.title ? `${item.company} — ${item.title}` : item.company;
}

function jobPath(id: string): string {
  return `/jobs/${encodeURIComponent(id)}`;
}

function ApplicationBlock({
  title,
  rows,
  detail,
}: {
  title: string;
  rows: TodayApplication[];
  detail: (r: TodayApplication) => string;
}) {
  if (rows.length === 0) return null;
  return (
    <section aria-label={title} className="mb-6">
      <h2>{title}</h2>
      <ul className="flex flex-col gap-2">
        {rows.map((row) => (
          <li
            key={row.jobListingId}
            className="flex flex-wrap items-baseline gap-x-3"
          >
            <Link
              to={jobPath(row.jobListingId)}
              className="font-medium no-underline hover:underline"
            >
              {heading(row)}
            </Link>
            <span className="text-sm text-muted-foreground">{detail(row)}</span>
          </li>
        ))}
      </ul>
    </section>
  );
}

function idleDetail(row: TodayApplication): string {
  const since = row.statusUpdatedAt ? new Date(row.statusUpdatedAt) : null;
  if (!since || Number.isNaN(since.getTime())) return row.status;
  return `${row.status} since ${since.toLocaleDateString()}`;
}

function isEmpty(t: Today): boolean {
  return (
    t.followUps.length === 0 &&
    t.interviews.length === 0 &&
    t.stale.length === 0 &&
    t.trackedBoards === 0 &&
    t.pendingCaptures === 0
  );
}

// TodayPage is the read-only "what needs action" view over GET /api/today:
// every block hides when empty, and all empty shows one empty state.
export default function TodayPage() {
  const [today, setToday] = useState<Today | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    getToday()
      .then(setToday)
      .catch((e) => setError(e.message));
  }, []);

  if (error)
    return (
      <p role="alert" className="font-medium text-destructive">
        {error}
      </p>
    );
  if (!today) return <p>Loading…</p>;

  return (
    <>
      <h1>Today</h1>
      {isEmpty(today) && <p>Nothing needs your attention today.</p>}
      <ApplicationBlock
        title="Follow-up"
        rows={today.followUps}
        detail={(r) => `follow-up ${r.followUpAt ?? ""}`}
      />
      {today.interviews.length > 0 && (
        <section aria-label="Interviews" className="mb-6">
          <h2>Interviews</h2>
          <ul className="flex flex-col gap-2">
            {today.interviews.map((iv) => (
              <li
                key={`${iv.jobListingId}-${iv.at}`}
                className="flex flex-wrap items-baseline gap-x-3"
              >
                <span className="text-sm">{iv.at.replace("T", " ")}</span>
                <Link
                  to={jobPath(iv.jobListingId)}
                  className="font-medium no-underline hover:underline"
                >
                  {heading(iv)}
                </Link>
                {iv.location && (
                  <span className="text-sm text-muted-foreground">
                    {iv.location}
                  </span>
                )}
              </li>
            ))}
          </ul>
        </section>
      )}
      <ApplicationBlock
        title="Stalled"
        rows={today.stale}
        detail={idleDetail}
      />
      {(today.trackedBoards > 0 || today.pendingCaptures > 0) && (
        <section aria-label="To look at" className="mb-6">
          <h2>To look at</h2>
          <ul className="flex flex-col gap-2">
            {today.trackedBoards > 0 && (
              <li>
                <Link to="/ats">
                  {today.trackedBoards} followed{" "}
                  {today.trackedBoards === 1 ? "board" : "boards"}
                </Link>
              </li>
            )}
            {today.pendingCaptures > 0 && (
              <li>
                <Link to="/inbox">
                  {today.pendingCaptures}{" "}
                  {today.pendingCaptures === 1 ? "capture" : "captures"} in the
                  Inbox
                </Link>
              </li>
            )}
          </ul>
        </section>
      )}
    </>
  );
}
