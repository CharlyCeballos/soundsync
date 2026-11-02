import type { Flow } from "../flow";
import { useT } from "../i18n";
import { Icon, Spinner } from "../components/Icon";
import { Banner, ExternalLink, YouTubeAttribution } from "../components/ui";
import type { Models } from "../api";
import { VideoTable } from "./Review";

/** Done is step 3, a YouTube view: the write as it happens, then what
 * changed, the report on demand and the way to copy more. */
export function Done({ flow }: { flow: Flow }) {
  const channel = flow.status?.youtube;
  const result = flow.result;

  return (
    <div className="page">
      {channel && (
        <div className="attribution">
          <YouTubeAttribution channelURL={channel.url} />
        </div>
      )}
      {flow.busy === "write" || !result ? <Writing flow={flow} /> : <Result flow={flow} result={result} />}
      {flow.error && <div style={{ marginTop: 16 }}><Banner tone="error">{flow.error}</Banner></div>}
    </div>
  );
}

function Writing({ flow }: { flow: Flow }) {
  const { t } = useT();
  const playlists = (flow.plan?.playlists ?? []).filter((p) => p.hasWrites);
  return (
    <>
      <section className="panel panel-pad stack" aria-live="polite">
        <div className="row"><Spinner /><h1>{t("writingTitle")}</h1></div>
        <ul className="stack">
          {playlists.map((p) => {
            const ev = flow.progress[p.sourceId];
            const writing = ev && (ev.step === "add" || ev.step === "remove");
            return (
              <li key={p.sourceId} className="stack" style={{ gap: 6 }}>
                <div className="row small">
                  <span className="item-title">{p.target.title}</span>
                  <span className="spacer" />
                  {writing && <span className="muted num">{ev.done}/{ev.total}</span>}
                </div>
                <progress max={writing ? ev.total || 1 : 1} value={writing ? ev.done : undefined} />
                {ev?.title && <span className="small muted clip" title={ev.title}>{ev.title}</span>}
              </li>
            );
          })}
        </ul>
      </section>
      <div className="actionbar">
        <span className="spacer" />
        <button type="button" className="btn btn-secondary btn-lg" onClick={flow.stop} disabled={flow.busy !== "write"}>
          <Icon name="stop" />
          {t("stop")}
        </button>
      </div>
    </>
  );
}

function Stat({ label, value }: { label: string; value: number }) {
  return (
    <div>
      <dt>{label}</dt>
      <dd className="num">{value}</dd>
    </div>
  );
}

function Result({ flow, result }: { flow: Flow; result: Models.RunView }) {
  const { t } = useT();
  const stopped = result.cancelled || !!result.error;
  return (
    <>
      <div className="page-head">
        <h1>{t(stopped ? "doneStoppedTitle" : "doneTitle")}</h1>
        {stopped && <p>{t("doneStoppedNote")}</p>}
      </div>
      {result.error && <div style={{ marginBottom: 16 }}><Banner tone="error">{result.error}</Banner></div>}

      <ul className="stack">
        {(result.playlists ?? []).map((p) => (
          <li key={p.sourceId} className="panel panel-pad stack">
            <div className="row-wrap">
              <h2 className="item-title" style={{ fontSize: "var(--fs-lg)" }}>{p.target.title}</h2>
              {p.inSync ? <span className="badge badge-ok"><Icon name="check" />{t("inSync")}</span>
                : p.error || result.cancelled ? <span className="badge badge-warn">{t("stopped")}</span> : null}
              <span className="spacer" />
              {p.ready && p.target.musicUrl && (
                <ExternalLink href={p.target.musicUrl} className="btn btn-secondary">
                  {t("openInMusic")}
                  <Icon name="external" />
                </ExternalLink>
              )}
            </div>
            <dl className="kv">
              <Stat label={t("statAdded")} value={p.added} />
              <Stat label={t("statPresent")} value={p.present} />
              <Stat label={t("statRemoved")} value={p.removed} />
              {p.keptYours > 0 && <Stat label={t("statKept")} value={p.keptYours} />}
              {p.noMatch > 0 && <Stat label={t("statNoMatch")} value={p.noMatch} />}
              {p.skipped > 0 && <Stat label={t("statSkipped")} value={p.skipped} />}
              {p.waiting > 0 && <Stat label={t("statWaiting")} value={p.waiting} />}
            </dl>
            {p.error && <Banner tone="error">{p.error}</Banner>}
          </li>
        ))}
      </ul>

      {(result.playlists ?? []).filter((p) => p.hasWrites).map((p) => <VideoTable key={p.sourceId} playlist={p} />)}

      <p className="small muted num" style={{ marginTop: 16 }}>
        {t("quotaUsed", { units: result.unitsUsed, perDay: result.unitsPerDay, searches: result.searchesUsed, searchesPerDay: result.searchesPerDay })}
      </p>

      <section className="panel panel-pad stack" style={{ marginTop: 16 }} aria-labelledby="report">
        <h2 id="report">{t("reportTitle")}</h2>
        <p className="muted">{t("reportBody")}</p>
        <div className="row-wrap">
          <button type="button" className="btn btn-secondary" onClick={() => void flow.saveReport("csv")} disabled={flow.busy !== null}>
            <Icon name="download" />
            {t("reportCSV")}
          </button>
          <button type="button" className="btn btn-secondary" onClick={() => void flow.saveReport("json")} disabled={flow.busy !== null}>
            <Icon name="download" />
            {t("reportJSON")}
          </button>
        </div>
        {flow.savedPath && <Banner tone="ok">{t("reportSaved", { path: flow.savedPath })}</Banner>}
      </section>

      <div className="actionbar">
        <span className="spacer" />
        <button type="button" className="btn btn-primary btn-lg" onClick={flow.startOver}>
          {t("copyMore")}
          <Icon name="arrowRight" />
        </button>
      </div>
    </>
  );
}
