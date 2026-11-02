import { useEffect } from "react";
import { useFlow, type Flow, type Step } from "./flow";
import { I18nProvider, useT, type Lang } from "./i18n";
import type { Key } from "./i18n/en";
import { AppMark, Icon, Spinner } from "./components/Icon";
import { ExternalLink } from "./components/ui";
import { Setup } from "./screens/Setup";
import { Privacy } from "./screens/Privacy";
import { Choose } from "./screens/Choose";
import { Review } from "./screens/Review";
import { Done } from "./screens/Done";

export function App() {
  const flow = useFlow();
  useEffect(() => {
    document.documentElement.lang = flow.lang;
  }, [flow.lang]);
  return (
    <I18nProvider lang={flow.lang}>
      <Frame flow={flow} />
    </I18nProvider>
  );
}

const steps: { step: Step; key: Key }[] = [
  { step: 1, key: "stepChoose" },
  { step: 2, key: "stepReview" },
  { step: 3, key: "stepDone" },
];

function Frame({ flow }: { flow: Flow }) {
  const { t } = useT();
  return (
    <div className="app">
      <header className="topbar">
        <div className="brand">
          <AppMark />
          <span>{t("appName")}</span>
        </div>
        {flow.gate === "flow" ? <Stepper flow={flow} /> : <span className="spacer" />}
        <LanguageSwitch lang={flow.lang} onChange={(l) => void flow.setLang(l)} />
      </header>

      {flow.update ? (
        <div className="update-bar" role="status">
          <span>{t("updateAvailable", { tag: flow.update.tag })}</span>
          <ExternalLink href={flow.update.url}>{t("updateDownload")}</ExternalLink>
          <button type="button" className="btn btn-ghost icon-btn" onClick={flow.dismissUpdate} aria-label={t("dismiss")}>
            <Icon name="x" />
          </button>
        </div>
      ) : <div />}

      <main>
        <Body flow={flow} />
      </main>

      <Footer flow={flow} />
    </div>
  );
}

function Body({ flow }: { flow: Flow }) {
  const { t } = useT();
  switch (flow.gate) {
    case "loading":
      return (
        <div className="center-panel" aria-busy>
          <Spinner label={t("loading")} />
          {flow.error && <p className="multiline">{t("unexpected", { message: flow.error })}</p>}
        </div>
      );
    case "setup":
      return <Setup flow={flow} />;
    case "privacy":
      return <Privacy flow={flow} />;
    case "flow":
      if (flow.step === 1) return <Choose flow={flow} />;
      if (flow.step === 2) return <Review flow={flow} />;
      return <Done flow={flow} />;
  }
}

function Stepper({ flow }: { flow: Flow }) {
  const { t } = useT();
  return (
    <nav aria-label={t("stepChoose") + " · " + t("stepReview") + " · " + t("stepDone")}>
      <ol className="stepper">
        {steps.map(({ step, key }) => {
          const current = flow.step === step;
          const done = step < flow.step;
          const label = t(key);
          const content = (
            <>
              <span className="dot">{done ? <Icon name="check" /> : step}</span>
              <span>{label}</span>
            </>
          );
          // Going back is allowed (it drops the later work); going forward
          // only happens through each step's own action.
          const canGoBack = done && flow.busy === null && flow.step !== 3;
          return (
            <li key={step}>
              {canGoBack ? (
                <button type="button" className="step done" onClick={() => flow.goTo(step)} aria-label={t("stepBack", { label })}>
                  {content}
                </button>
              ) : (
                <span className={`step ${done ? "done" : ""}`} aria-current={current ? "step" : undefined}>
                  {content}
                  {current && <span className="visually-hidden"> ({t("stepCurrent")})</span>}
                </span>
              )}
            </li>
          );
        })}
      </ol>
    </nav>
  );
}

function LanguageSwitch({ lang, onChange }: { lang: Lang; onChange: (l: Lang) => void }) {
  const { t } = useT();
  return (
    <div className="lang" role="group" aria-label={t("language")}>
      {(["es", "en"] as const).map((l) => (
        <button key={l} type="button" aria-pressed={lang === l} onClick={() => onChange(l)} lang={l}>
          {l === "es" ? "ES" : "EN"}
          <span className="visually-hidden">{l === "es" ? " Español" : " English"}</span>
        </button>
      ))}
    </div>
  );
}

function Footer({ flow }: { flow: Flow }) {
  const { t } = useT();
  const links = flow.status?.links;
  return (
    <footer className="footer">
      {links && (
        <>
          <ExternalLink href={links.policy}>{t("footerPrivacy")}</ExternalLink>
          <ExternalLink href={links.youtubeTerms}>{t("footerTerms")}</ExternalLink>
          <ExternalLink href={links.googlePrivacy}>{t("footerGooglePrivacy")}</ExternalLink>
          <ExternalLink href={links.googlePermissions}>{t("footerRevoke")}</ExternalLink>
        </>
      )}
      {flow.status && <span className="version num">{t("footerVersion", { version: flow.status.version })}</span>}
    </footer>
  );
}
