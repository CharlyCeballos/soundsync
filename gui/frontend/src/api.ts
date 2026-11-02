// The backend as the frontend sees it: the Go services' generated bindings,
// their models, and the two events the backend sends.
import { CancelError, Events } from "@wailsio/runtime";
import * as Settings from "../bindings/github.com/CharlyCeballos/soundsync/internal/desktop/settings";
import * as Accounts from "../bindings/github.com/CharlyCeballos/soundsync/internal/desktop/accounts";
import * as Copy from "../bindings/github.com/CharlyCeballos/soundsync/internal/desktop/copy";
import type * as Models from "../bindings/github.com/CharlyCeballos/soundsync/internal/desktop/models";
import { Status as RowStatus, type Row } from "../bindings/github.com/CharlyCeballos/soundsync/internal/report/models";
import type { Key } from "./i18n/en";

export { Settings, Accounts, Copy, RowStatus };
export type { Models, Row };

export type Privacy = "private" | "unlisted" | "public";

/** isCancel tells a call the user stopped from a real failure. */
export function isCancel(err: unknown): boolean {
  return err instanceof CancelError;
}

/** messageOf is the text to show for a failed call: the backend already
 * translated it and may add a hint on the following lines. */
export function messageOf(err: unknown): string {
  if (err instanceof Error) return err.message;
  return String(err);
}

export type LoginURL = { provider: "spotify" | "youtube"; url: string };
export type Progress = { sourceId: string; step: string; done: number; total: number; title?: string };

/** onEvent subscribes to a backend event and returns the unsubscribe. */
export function onLoginURL(handle: (e: LoginURL) => void): () => void {
  return Events.On("login:url", (ev) => handle(ev.data as LoginURL));
}

export function onProgress(handle: (p: Progress) => void): () => void {
  return Events.On("copy:progress", (ev) => handle(ev.data as Progress));
}

/** statusKey names every report status; the compiler requires them all. */
export const statusKey: Record<Exclude<RowStatus, RowStatus.$zero>, Key> = {
  [RowStatus.Added]: "status_added",
  [RowStatus.ToAdd]: "status_to add (not done)",
  [RowStatus.AlreadyThere]: "status_already there",
  [RowStatus.SameVideo]: "status_same video as an earlier track",
  [RowStatus.NoMatch]: "status_no match",
  [RowStatus.LeftOut]: "status_left out: playlist full",
  [RowStatus.KeptYours]: "status_kept yours",
  [RowStatus.NotSearchedFailed]: "status_not searched: the run stopped",
  [RowStatus.Waiting]: "status_waiting: no searches left",
  [RowStatus.Episode]: "status_not searchable: podcast episode",
  [RowStatus.Unavailable]: "status_not searchable: unavailable",
  [RowStatus.Removed]: "status_removed",
  [RowStatus.ToRemove]: "status_to remove (not done)",
};

const whyKeys: Record<string, Key> = {
  topic: "why_topic",
  vevo: "why_vevo",
  first: "why_first",
  playlist: "why_playlist",
  yours: "why_yours",
};

export function whyKey(why: string): Key | undefined {
  return whyKeys[why];
}

export function privacyShortKey(p: string): Key {
  switch (p) {
    case "unlisted":
      return "privacyShort_unlisted";
    case "public":
      return "privacyShort_public";
    default:
      return "privacyShort_private";
  }
}
