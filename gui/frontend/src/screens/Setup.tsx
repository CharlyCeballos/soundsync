import { useState } from "react";
import type { Flow } from "../flow";
import { useT } from "../i18n";
import type { Key } from "../i18n/en";
import { Icon } from "../components/Icon";
import { Banner, ExternalLink } from "../components/ui";

const stateLabel: Record<string, Key> = { set: "setupStateSet", missing: "setupStateMissing", invalid: "setupStateInvalid" };
const stateTone: Record<string, string> = { set: "badge-ok", missing: "badge-warn", invalid: "badge-danger" };

/** Setup is the first gate: the user's own OAuth clients in .env. */
export function Setup({ flow }: { flow: Flow }) {
  const { t } = useT();
  const [checking, setChecking] = useState(false);
  const status = flow.status!;
  const config = status.config;

  const recheck = async () => {
    setChecking(true);
    await flow.refreshStatus();
    setChecking(false);
  };

  return (
    <div className="page">
      <div className="page-head">
        <h1>{t("setupTitle")}</h1>
        <p>{t("setupLead")}</p>
      </div>

      <div className="panel panel-pad stack-lg">
        <ol className="steps-list">
          <li>
            <div className="stack">
              <h3>{t("setupStep1")}</h3>
              <div>
                <ExternalLink href={status.links.setupGuide} className="btn btn-secondary">
                  {t("setupGuide")}
                  <Icon name="external" />
                </ExternalLink>
              </div>
            </div>
          </li>
          <li>
            <div className="stack">
              <h3>{t("setupStep2")}</h3>
              <div className="small">
                <span className="muted">{t("setupFile")}: </span>
                {config.found ? <span className="code">{config.file}</span> : <span className="muted">{t("setupNoFile")}</span>}
              </div>
              <div className="row-wrap">
                {config.found ? (
                  <button type="button" className="btn btn-secondary" onClick={() => void flow.openConfigFolder()}>
                    <Icon name="folder" />
                    {t("setupOpenFolder")}
                  </button>
                ) : (
                  <button type="button" className="btn btn-primary" onClick={() => void flow.createConfig()}>
                    <Icon name="file" />
                    {t("setupCreate")}
                  </button>
                )}
              </div>
              {config.found && <p className="small muted">{t("setupCreated")}</p>}
            </div>
          </li>
          <li>
            <div className="stack">
              <h3>{t("setupStep3")}</h3>
              <p className="small muted">{t("setupStep3Help")}</p>
              <div>
                <button type="button" className="btn btn-secondary" onClick={() => void recheck()} disabled={checking}>
                  <Icon name="refresh" />
                  {t("setupRecheck")}
                </button>
              </div>
            </div>
          </li>
        </ol>

        {config.found && config.fields && (
          <section aria-labelledby="found">
            <h3 id="found" className="section-title">{t("setupFound")}</h3>
            <ul className="list panel" style={{ boxShadow: "none" }}>
              {config.fields.map((f) => (
                <li key={f.name}>
                  <span className="code" style={{ flex: 1 }}>{f.name}</span>
                  <span className={`badge ${stateTone[f.state] ?? ""}`}>{t(stateLabel[f.state] ?? "setupStateInvalid")}</span>
                </li>
              ))}
            </ul>
          </section>
        )}

        {config.found && (config.problems?.length ?? 0) > 0 && (
          <details>
            <summary className="link small">{t("setupDetails")}</summary>
            <ul className="small muted" style={{ marginTop: 8 }}>
              {config.problems!.map((p) => (
                <li key={p} className="code">{p}</li>
              ))}
            </ul>
          </details>
        )}
        {flow.error && <Banner tone="error">{flow.error}</Banner>}
      </div>
    </div>
  );
}
