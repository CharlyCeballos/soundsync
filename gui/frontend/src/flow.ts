import { useCallback, useEffect, useRef, useState } from "react";
import type { CancellablePromise } from "@wailsio/runtime";
import {
  Accounts, Copy, Settings, isCancel, messageOf, onLoginURL, onProgress,
  type LoginURL, type Models, type Privacy, type Progress,
} from "./api";
import { asLang, type Lang } from "./i18n";

export type Gate = "loading" | "setup" | "privacy" | "flow";
export type Step = 1 | 2 | 3;
export type Busy = null | "spotify" | "youtube" | "playlists" | "resolve" | "plan" | "write" | "report";

const LANG_KEY = "soundsync.lang";

function storedLang(): Lang | null {
  try {
    const v = localStorage.getItem(LANG_KEY);
    return v === "es" || v === "en" ? v : null;
  } catch {
    return null; // storage may be unavailable; the system language is used
  }
}

function gateOf(status: Models.Status | null): Gate {
  if (!status) return "loading";
  if (!status.config.ok) return "setup";
  if (!status.policyAccepted) return "privacy";
  return "flow";
}

/** useFlow holds the wizard: what is connected, chosen, planned and written. */
export function useFlow() {
  const [status, setStatus] = useState<Models.Status | null>(null);
  const [lang, setLangState] = useState<Lang>(storedLang() ?? "es");
  const [update, setUpdate] = useState<Models.Release | null>(null);
  const [step, setStep] = useState<Step>(1);
  const [playlists, setPlaylists] = useState<Models.SourcePlaylist[] | null>(null);
  const [selected, setSelected] = useState<string[]>([]);
  const [dests, setDests] = useState<Models.DestinationView[] | null>(null);
  const [adopt, setAdoptState] = useState<Record<string, string | undefined>>({});
  const [privacy, setPrivacy] = useState<Privacy>("private");
  const [plan, setPlan] = useState<Models.RunView | null>(null);
  const [result, setResult] = useState<Models.RunView | null>(null);
  const [progress, setProgress] = useState<Record<string, Progress>>({});
  const [busy, setBusy] = useState<Busy>(null);
  const [login, setLogin] = useState<LoginURL | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [stoppedNote, setStoppedNote] = useState(false);
  const [savedPath, setSavedPath] = useState<string | null>(null);
  const pending = useRef<CancellablePromise<unknown> | null>(null);

  /** run tracks one backend call: busy state, cancel handle, errors. */
  const run = useCallback(async <T,>(kind: Exclude<Busy, null>, call: () => CancellablePromise<T>): Promise<T | undefined> => {
    setBusy(kind);
    setError(null);
    const p = call();
    pending.current = p as CancellablePromise<unknown>;
    try {
      return await p;
    } catch (err) {
      if (!isCancel(err)) setError(messageOf(err));
      return undefined;
    } finally {
      pending.current = null;
      setBusy(null);
      setLogin(null);
    }
  }, []);

  const refreshStatus = useCallback(async () => {
    try {
      const s = await Settings.Status();
      setStatus(s);
      return s;
    } catch (err) {
      setError(messageOf(err));
      return null;
    }
  }, []);

  // First load: status, language, update notice, backend events.
  useEffect(() => {
    void (async () => {
      const s = await refreshStatus();
      const chosen = storedLang() ?? asLang(s?.language);
      setLangState(chosen);
      await Settings.SetLanguage(chosen).catch(() => undefined);
      const rel = await Settings.CheckUpdate().catch(() => null);
      setUpdate(rel ?? null);
    })();
    const offLogin = onLoginURL(setLogin);
    const offProgress = onProgress((p) => setProgress((cur) => ({ ...cur, [p.sourceId]: p })));
    return () => {
      offLogin();
      offProgress();
    };
  }, [refreshStatus]);

  const gate = gateOf(status);

  // While the user edits .env, check again whenever they come back.
  useEffect(() => {
    if (gate !== "setup") return undefined;
    const onFocus = () => void refreshStatus();
    window.addEventListener("focus", onFocus);
    return () => window.removeEventListener("focus", onFocus);
  }, [gate, refreshStatus]);

  const setLang = useCallback(async (l: Lang) => {
    setLangState(l);
    try {
      localStorage.setItem(LANG_KEY, l);
    } catch {
      // not remembered: fine, the system language applies next time
    }
    await Settings.SetLanguage(l).catch((err: unknown) => setError(messageOf(err)));
  }, []);

  const loadPlaylists = useCallback(async (refresh: boolean) => {
    const list = await run("playlists", () => Accounts.Playlists(refresh));
    if (list) setPlaylists(list);
  }, [run]);

  const resetCopy = useCallback(() => {
    setDests(null);
    setAdoptState({});
    setPlan(null);
    setResult(null);
    setProgress({});
    setStoppedNote(false);
    setSavedPath(null);
  }, []);

  const resolve = useCallback(async (ids: string[]) => {
    resetCopy();
    const views = await run("resolve", () => Copy.Resolve(ids));
    if (views) setDests(views);
  }, [run, resetCopy]);

  const actions = {
    refreshStatus,
    setLang,
    dismissUpdate: () => setUpdate(null),
    clearError: () => setError(null),

    async acceptPolicy() {
      await Settings.AcceptPolicy().catch((err: unknown) => setError(messageOf(err)));
      await refreshStatus();
    },
    async createConfig() {
      await Settings.CreateConfigFile().catch((err: unknown) => setError(messageOf(err)));
      await refreshStatus();
    },
    openConfigFolder: () => Settings.OpenConfigFolder().catch((err: unknown) => setError(messageOf(err))),

    /** cancel abandons a login the user will not finish in the browser. */
    cancel() {
      pending.current?.cancel();
    },
    /** stop ends a search or a write early; it still returns what it did. */
    stop() {
      void Copy.Stop();
    },

    async connectSpotify() {
      const acc = await run("spotify", () => Accounts.ConnectSpotify());
      if (!acc) return;
      await refreshStatus();
      await loadPlaylists(false);
    },
    async disconnectSpotify() {
      await Accounts.DisconnectSpotify().catch((err: unknown) => setError(messageOf(err)));
      // Forget the account before the list: with the account still known and
      // no list, step 1 would ask for the playlists of a session that is gone.
      setStatus((s) => (s ? { ...s, spotify: null } : s));
      setPlaylists(null);
      setSelected([]);
      resetCopy();
      setStep(1);
      await refreshStatus();
    },
    loadPlaylists,
    toggle(id: string) {
      setSelected((cur) => (cur.includes(id) ? cur.filter((x) => x !== id) : [...cur, id]));
    },

    async review() {
      setStep(2);
      if (status?.youtube) await resolve(selected);
    },
    async connectYouTube() {
      const ch = await run("youtube", () => Accounts.ConnectYouTube());
      if (!ch) return;
      await refreshStatus();
      await resolve(selected);
    },
    async disconnectYouTube() {
      await Accounts.DisconnectYouTube().catch((err: unknown) => setError(messageOf(err)));
      setStatus((s) => (s ? { ...s, youtube: null } : s));
      resetCopy();
      await refreshStatus();
    },
    setAdopt(sourceId: string, playlistId: string) {
      setAdoptState((cur) => ({ ...cur, [sourceId]: playlistId }));
    },
    setPrivacy,

    async findSongs() {
      setProgress({});
      setStoppedNote(false);
      const answers: Record<string, string> = {};
      for (const [k, v] of Object.entries(adopt)) if (v !== undefined) answers[k] = v;
      const view = await run("plan", () => Copy.Plan({ adopt: answers, privacy }));
      if (!view) return;
      setPlan(view);
      setStoppedNote(view.cancelled);
    },

    async write() {
      setStep(3);
      setProgress({});
      const view = await run("write", () => Copy.Write());
      if (view) setResult(view);
    },

    async saveReport(format: "csv" | "json") {
      setSavedPath(null);
      const path = await run("report", () => Copy.SaveReport(format));
      if (path) setSavedPath(path);
    },

    goTo(target: Step) {
      if (target >= step || busy) return;
      setStep(target);
      if (target === 1) resetCopy();
    },
    startOver() {
      resetCopy();
      setSelected([]);
      setStep(1);
    },
  };

  return {
    gate, status, lang, update, step, playlists, selected, dests, adopt, privacy,
    plan, result, progress, busy, login, error, stoppedNote, savedPath,
    ...actions,
  };
}

export type Flow = ReturnType<typeof useFlow>;
