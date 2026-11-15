import { useEffect, useRef, useState } from "react";
import { brandOnScreen, type Brand, type Flow } from "../flow";
import { useT, type Lang } from "../i18n";
import { Icon, Spinner } from "./Icon";
import { SpotifyAttribution, YouTubeAttribution } from "./ui";

const PANEL_ID = "options";
const brands: Brand[] = ["spotify", "youtube"];
const brandName: Record<Brand, string> = { spotify: "Spotify", youtube: "YouTube" };
// Each language is named in itself, whatever the interface language.
const languages: { lang: Lang; name: string }[] = [
  { lang: "es", name: "Español" },
  { lang: "en", name: "English" },
];

/** OptionsButton opens and closes the options panel (a native popover
 * invoker: the browser keeps its expanded state and returns focus). */
export function OptionsButton() {
  const { t } = useT();
  return (
    <button type="button" className="btn btn-secondary icon-btn options-toggle" popovertarget={PANEL_ID} aria-label={t("options")} title={t("options")}>
      <Icon name="settings" />
    </button>
  );
}

/** OptionsPanel slides in beside the app without blocking it, so the app
 * can be seen reacting to what changes here. Esc or a click outside closes it. */
export function OptionsPanel({ flow }: { flow: Flow }) {
  const { t } = useT();
  const panel = useRef<HTMLElement>(null);
  const heading = useRef<HTMLHeadingElement>(null);

  useEffect(() => {
    const el = panel.current;
    if (!el) return undefined;
    const onToggle = (e: Event) => {
      if ((e as ToggleEvent).newState === "open") heading.current?.focus();
    };
    el.addEventListener("toggle", onToggle);
    return () => el.removeEventListener("toggle", onToggle);
  }, []);

  return (
    <aside ref={panel} id={PANEL_ID} popover="auto" className="options" aria-labelledby="options-title">
      <div className="options-head">
        <h2 id="options-title" ref={heading} tabIndex={-1}>{t("options")}</h2>
        <button type="button" className="btn btn-ghost icon-btn" popovertarget={PANEL_ID} popovertargetaction="hide" aria-label={t("optionsClose")}>
          <Icon name="x" />
        </button>
      </div>

      <AccountsSection flow={flow} />

      <section className="options-section" aria-labelledby="options-language">
        <h3 id="options-language">{t("language")}</h3>
        <div className="segmented" role="group" aria-labelledby="options-language">
          {languages.map(({ lang, name }) => (
            <button key={lang} type="button" lang={lang} aria-pressed={flow.lang === lang} onClick={() => void flow.setLang(lang)}>
              {name}
            </button>
          ))}
        </div>
      </section>

      <ConfigSection flow={flow} />
    </aside>
  );
}

/** AccountsSection shows in full only the platform whose mark is already on
 * screen; the other one gets a line of plain text, no mark and no account
 * name, so the two brands never share the view (docs/compliance.md). */
function AccountsSection({ flow }: { flow: Flow }) {
  const { t } = useT();
  const shown = brandOnScreen(flow);
  return (
    <section className="options-section" aria-labelledby="options-accounts">
      <h3 id="options-accounts">{t("accounts")}</h3>
      {shown === "spotify" && <SpotifyCard flow={flow} />}
      {shown === "youtube" && <YouTubeCard flow={flow} />}
      <ul className="account-lines">
        {brands.filter((b) => b !== shown).map((b) => <AccountLine key={b} flow={flow} brand={b} />)}
      </ul>
    </section>
  );
}

function SpotifyCard({ flow }: { flow: Flow }) {
  const { t } = useT();
  const account = flow.status?.spotify;
  return (
    <div className="account-card">
      <SpotifyAttribution />
      {account ? (
        <>
          <p>{t("connectedAs", { name: account.name })}</p>
          <Disconnect flow={flow} brand="spotify" />
        </>
      ) : (
        <NotConnected brand="spotify" />
      )}
    </div>
  );
}

function YouTubeCard({ flow }: { flow: Flow }) {
  const { t } = useT();
  const channel = flow.status?.youtube;
  if (!channel) return null; // brandOnScreen only names YouTube once it is connected
  return (
    <div className="account-card">
      <YouTubeAttribution channelURL={channel.url} />
      <p>{t("connectedAs", { name: channel.title })}</p>
      <Disconnect flow={flow} brand="youtube" />
    </div>
  );
}

function AccountLine({ flow, brand }: { flow: Flow; brand: Brand }) {
  const { t } = useT();
  const connected = brand === "spotify" ? !!flow.status?.spotify : !!flow.status?.youtube;
  return (
    <li className="account-line">
      <p className="account-state">
        <span className={connected ? "dot-on" : "dot-off"} aria-hidden />
        {brandName[brand]} · {t(connected ? "connected" : "notConnected")}
      </p>
      {connected ? <Disconnect flow={flow} brand={brand} /> : <ConnectHint brand={brand} />}
    </li>
  );
}

function NotConnected({ brand }: { brand: Brand }) {
  const { t } = useT();
  return (
    <>
      <p className="account-state">
        <span className="dot-off" aria-hidden />
        {t("notConnected")}
      </p>
      <ConnectHint brand={brand} />
    </>
  );
}

function ConnectHint({ brand }: { brand: Brand }) {
  const { t } = useT();
  const step = brand === "spotify" ? { n: 1, label: t("stepChoose") } : { n: 2, label: t("stepReview") };
  return <p className="small muted">{t("connectInStep", step)}</p>;
}

/** Disconnect waits while another step runs: the backend would refuse. */
function Disconnect({ flow, brand }: { flow: Flow; brand: Brand }) {
  const { t } = useT();
  const [pending, setPending] = useState(false);
  const blocked = flow.busy !== null && !pending;

  const disconnect = async () => {
    setPending(true);
    try {
      await (brand === "spotify" ? flow.disconnectSpotify() : flow.disconnectYouTube());
    } finally {
      setPending(false);
    }
  };

  return (
    <div className="account-action">
      <button
        type="button"
        className="btn btn-secondary"
        onClick={() => void disconnect()}
        disabled={blocked || pending}
        aria-busy={pending}
        aria-label={`${t("disconnect")} ${brandName[brand]}`}
      >
        {pending && <Spinner />}
        {t("disconnect")}
      </button>
      {blocked && <p className="small muted">{t("disconnectBusy")}</p>}
    </div>
  );
}

function ConfigSection({ flow }: { flow: Flow }) {
  const { t } = useT();
  const [checking, setChecking] = useState(false);
  const [checked, setChecked] = useState(false);
  const config = flow.status?.config;

  const recheck = async () => {
    setChecking(true);
    setChecked(false);
    await flow.refreshStatus();
    setChecking(false);
    setChecked(true);
  };

  const state = !config?.found
    ? { tone: "badge-warn", key: "setupNoFile" as const }
    : config.ok
      ? { tone: "badge-ok", key: "configReady" as const }
      : { tone: "badge-warn", key: "configIncomplete" as const };

  return (
    <section className="options-section" aria-labelledby="options-config">
      <div className="options-row">
        <h3 id="options-config">{t("setupFile")}</h3>
        <span className={`badge ${state.tone}`}>{t(state.key)}</span>
      </div>
      {config?.file && <p className="path">{config.file}</p>}
      <div className="row-wrap">
        <button type="button" className="btn btn-secondary" onClick={() => void flow.openConfigFile()} disabled={!config?.found}>
          <Icon name="file" />
          {t("setupOpenFile")}
        </button>
        <button type="button" className="btn btn-ghost" onClick={() => void recheck()} disabled={checking}>
          {checking ? <Spinner /> : <Icon name="refresh" />}
          {t("setupRecheck")}
        </button>
        <span className="small muted" role="status">
          {checked && !checking ? t("configChecked") : ""}
        </span>
      </div>
    </section>
  );
}
