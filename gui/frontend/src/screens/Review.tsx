import { useState } from "react";
import type { Flow } from "../flow";
import { useT, type Translate } from "../i18n";
import { Icon, Spinner } from "../components/Icon";
import { Banner, ExternalLink, Pager, SpotifyAttribution, YouTubeAttribution, pageOf } from "../components/ui";
import { RowStatus, privacyShortKey, statusKey, whyKey, type Models, type Privacy, type Row } from "../api";
import { Connect } from "./Connect";

type View = "youtube" | "spotify";

/** Review is step 2. By default everything here is the YouTube view; the
 * Spotify view swaps the whole region, logo included, so the two brands
 * never share the screen. Writes are only offered from the YouTube view,
 * where the exact changes are listed. */
export function Review({ flow }: { flow: Flow }) {
  const { t } = useT();
  const [view, setView] = useState<View>("youtube");
  const channel = flow.status?.youtube;

  if (!channel) return <div className="page"><Connect flow={flow} provider="youtube" /></div>;

  const plan = flow.plan && !flow.plan.cancelled ? flow.plan : null;
  const showSpotify = plan !== null && view === "spotify";

  return (
    <div className="page">
      <div className="attribution">
        {showSpotify ? <SpotifyAttribution /> : <YouTubeAttribution channelURL={channel.url} />}
        {!showSpotify && (
          <div className="row small">
            <span className="muted">{t("connectedAs", { name: channel.title })}</span>
            <button type="button" className="btn btn-ghost" onClick={() => void flow.disconnectYouTube()} disabled={flow.busy !== null}>
              {t("disconnect")}
            </button>
          </div>
        )}
      </div>

      <div className="page-head">
        <h1>{t(showSpotify ? "songsTitle" : "reviewTitle")}</h1>
        {!showSpotify && <p>{t("channelLine", { channel: channel.title })}</p>}
      </div>

      {flow.error && <div style={{ marginBottom: 16 }}><Banner tone="error">{flow.error}</Banner></div>}

      {flow.busy === "resolve" || (flow.dests === null && !flow.error) ? (
        <div className="panel panel-pad row"><Spinner /><span>{t("loading")}</span></div>
      ) : flow.busy === "plan" ? (
        <Searching flow={flow} />
      ) : plan ? (
        <>
          <div className="row" style={{ marginBottom: 16 }}>
            <span className="label">{t("showOn")}</span>
            <div className="segmented" role="group" aria-label={t("showOn")}>
              <button type="button" aria-pressed={view === "youtube"} onClick={() => setView("youtube")}>{t("showYouTube")}</button>
              <button type="button" aria-pressed={view === "spotify"} onClick={() => setView("spotify")}>{t("showSpotify")}</button>
            </div>
          </div>
          {view === "youtube" ? <YouTubePlan flow={flow} plan={plan} /> : <SpotifyPlan flow={flow} plan={plan} onBack={() => setView("youtube")} />}
        </>
      ) : (
        flow.dests && <Destinations flow={flow} />
      )}
    </div>
  );
}

function Destinations({ flow }: { flow: Flow }) {
  const { t } = useT();
  const dests = flow.dests!;
  const unanswered = dests.some((d) => d.kind === "choose" && flow.adopt[d.sourceId] === undefined);
  const mayCreate = dests.some((d) => d.kind !== "existing");

  return (
    <>
      {flow.stoppedNote && <div style={{ marginBottom: 16 }}><Banner tone="info">{t("stoppedNothingWritten")}</Banner></div>}
      <section className="panel" aria-labelledby="dest-title">
        <h2 id="dest-title" className="panel-head">{t("destinationsTitle")}</h2>
        <div>
          {dests.map((d) => (
            <div key={d.sourceId} className="dest">
              <div className="row-wrap">
                <span className="item-title" title={d.sourceName}>{d.target?.title ?? d.sourceName}</span>
                {d.kind === "existing" && d.target && (
                  <>
                    <span className="badge">{t("destExisting")}</span>
                    <span className="small muted num">{t("videos", { count: d.target.videos })}</span>
                    <ExternalLink href={d.target.url} className="small">{t("viewOnYouTube")}</ExternalLink>
                  </>
                )}
                {d.kind === "create" && <span className="badge badge-accent">{t("destNew")}</span>}
              </div>
              {d.kind === "choose" && <Choice flow={flow} dest={d} />}
            </div>
          ))}
        </div>
      </section>

      {mayCreate && (
        <div className="field" style={{ marginTop: 24, maxWidth: 360 }}>
          <label htmlFor="privacy">{t("privacyLabel")}</label>
          <select id="privacy" className="select" value={flow.privacy} onChange={(e) => flow.setPrivacy(e.target.value as Privacy)}>
            <option value="private">{t("privacy_private")}</option>
            <option value="unlisted">{t("privacy_unlisted")}</option>
            <option value="public">{t("privacy_public")}</option>
          </select>
        </div>
      )}

      <div className="actionbar">
        <span className="small muted" style={{ maxWidth: "60ch" }}>{unanswered ? t("answerFirst") : t("findSongsNote")}</span>
        <span className="spacer" />
        <button type="button" className="btn btn-primary btn-lg" disabled={unanswered || flow.busy !== null} onClick={() => void flow.findSongs()}>
          <Icon name="search" />
          {t("findSongs")}
        </button>
      </div>
    </>
  );
}

/** Choice asks which same-title playlist to use (todo 4.4), inline. */
function Choice({ flow, dest }: { flow: Flow; dest: Models.DestinationView }) {
  const { t } = useT();
  const name = `adopt-${dest.sourceId}`;
  const answer = flow.adopt[dest.sourceId];
  return (
    <div className="choice" role="radiogroup" aria-labelledby={`${name}-q`}>
      <p id={`${name}-q`} className="small" style={{ fontWeight: 600 }}>{t("destChoose")}</p>
      {(dest.candidates ?? []).map((c) => (
        <label key={c.id}>
          <input type="radio" name={name} checked={answer === c.id} onChange={() => flow.setAdopt(dest.sourceId, c.id)} />
          <span>
            {t("destUse", { title: c.title })}{" "}
            <span className="muted small">({t(privacyShortKey(c.privacy))}, <span className="num">{t("videos", { count: c.videos })}</span>)</span>{" "}
            <ExternalLink href={c.url} className="small">{t("viewOnYouTube")}</ExternalLink>
          </span>
        </label>
      ))}
      <label>
        <input type="radio" name={name} checked={answer === ""} onChange={() => flow.setAdopt(dest.sourceId, "")} />
        <span>{t("destCreateNew")}</span>
      </label>
      <p className="small muted">{t("destChooseHelp")}</p>
    </div>
  );
}

function Searching({ flow }: { flow: Flow }) {
  const { t } = useT();
  return (
    <>
      <section className="panel panel-pad stack" aria-live="polite">
        <div className="row"><Spinner /><h2>{t("searching")}</h2></div>
        <ul className="stack">
          {(flow.dests ?? []).map((d) => {
            const p = flow.progress[d.sourceId];
            const searching = p?.step === "search";
            return (
              <li key={d.sourceId} className="stack" style={{ gap: 6 }}>
                <div className="row small">
                  <span className="item-title">{d.target?.title ?? d.sourceName}</span>
                  <span className="spacer" />
                  {searching && <span className="muted num">{t("progressSongs", { done: p.done, total: p.total })}</span>}
                </div>
                <progress max={searching ? p.total || 1 : 1} value={searching ? p.done : undefined} />
              </li>
            );
          })}
        </ul>
      </section>
      <div className="actionbar">
        <span className="spacer" />
        <button type="button" className="btn btn-secondary btn-lg" onClick={flow.stop}>
          <Icon name="stop" />
          {t("stop")}
        </button>
      </div>
    </>
  );
}

function actionsOf(t: Translate, p: Models.PlaylistRun, privacy: string): string[] {
  const out: string[] = [];
  if (p.kind === "create") out.push(t("actionCreate", { title: p.target.title, privacy: t(privacyShortKey(privacy)) }));
  if (p.kind === "adopt") out.push(t("actionAdopt", { title: p.target.title }));
  if (p.add > 0) out.push(t("actionAdd", { count: p.add }));
  if (p.remove > 0) out.push(t("actionRemove", { count: p.remove }));
  return out;
}

function Notes({ p }: { p: Models.PlaylistRun }) {
  const { t } = useT();
  return (
    <>
      {p.waiting > 0 && <Banner tone="warn">{t("waitingNote", { count: p.waiting })}</Banner>}
      {p.leftOut > 0 && <Banner tone="warn">{t("leftOutNote", { count: p.leftOut })}</Banner>}
      {p.error && <Banner tone="error">{p.error}</Banner>}
    </>
  );
}

function YouTubePlan({ flow, plan }: { flow: Flow; plan: Models.RunView }) {
  const { t } = useT();
  const playlists = plan.playlists ?? [];
  return (
    <>
      <section className="panel panel-pad stack" aria-labelledby="changes">
        <h2 id="changes">{t("changesTitle", { channel: plan.channel.title })}</h2>
        {plan.searchesExhausted && <Banner tone="warn">{t("exhaustedNote")}</Banner>}
        {plan.error && <Banner tone="error">{plan.error}</Banner>}
        <ul className="stack">
          {playlists.map((p) => {
            const acts = actionsOf(t, p, flow.privacy);
            return (
              <li key={p.sourceId} className="stack" style={{ gap: 6 }}>
                <div className="row-wrap">
                  <span className="item-title">{p.target.title}</span>
                  {acts.length === 0 && <span className="badge badge-ok"><Icon name="check" />{t("upToDate")}</span>}
                </div>
                {acts.length > 0 && (
                  <ul className="small" style={{ paddingLeft: 18, listStyle: "disc" }}>
                    {acts.map((a) => <li key={a}>{a}</li>)}
                  </ul>
                )}
                <Notes p={p} />
              </li>
            );
          })}
        </ul>
        {plan.units > 0 && <p className="small muted num">{t("cost", { units: plan.units, perDay: plan.unitsPerDay, used: plan.unitsUsed })}</p>}
      </section>

      {playlists.map((p) => <VideoTable key={p.sourceId} playlist={p} />)}

      <div className="actionbar">
        {plan.changing > 0 ? (
          <>
            <span className="small muted">{t("consentNote")}</span>
            <span className="spacer" />
            <button type="button" className="btn btn-secondary btn-lg" onClick={() => flow.goTo(1)}>{t("back")}</button>
            <button type="button" className="btn btn-primary btn-lg" onClick={() => void flow.write()} disabled={flow.busy !== null}>
              {t("writeToYouTube")}
            </button>
          </>
        ) : (
          <>
            <span>{t("nothingToWrite")}</span>
            <span className="spacer" />
            <button type="button" className="btn btn-primary btn-lg" onClick={flow.startOver}>{t("copyMore")}</button>
          </>
        )}
      </div>
    </>
  );
}

/** VideoTable lists one playlist's YouTube videos: added, kept, removed. */
export function VideoTable({ playlist }: { playlist: Models.PlaylistRun }) {
  const { t } = useT();
  const [page, setPage] = useState(0);
  const rows = (playlist.rows ?? []).filter((r) => r.youtube_video_url);
  return (
    <section className="panel" style={{ marginTop: 16 }} aria-label={playlist.target.title}>
      <h3 className="panel-pad" style={{ paddingBottom: 8 }}>{playlist.target.title}</h3>
      {rows.length === 0 ? (
        <p className="panel-pad muted small" style={{ paddingTop: 0 }}>{t("noVideoRows")}</p>
      ) : (
        <>
          <div className="table-wrap">
            <table>
              <thead>
                <tr><th className="w-pos">{t("colNumber")}</th><th className="w-main">{t("colVideo")}</th><th className="w-status">{t("colStatus")}</th><th className="w-why">{t("colWhy")}</th><th /></tr>
              </thead>
              <tbody>
                {pageOf(rows, page).map((r, i) => (
                  <tr key={`${r.youtube_video_url}-${i}`}>
                    <td className="pos num">{r.position ?? "—"}</td>
                    <td>
                      <span className="clip" title={r.youtube_title}>{r.youtube_title}</span>
                      <span className="sub">{r.youtube_channel}</span>
                    </td>
                    <td><StatusBadge status={r.status} /></td>
                    <td className="muted">{why(t, r)}</td>
                    <td><ExternalLink href={r.youtube_video_url!} className="small">{t("watchOnYouTube")}</ExternalLink></td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          <Pager page={page} total={rows.length} onPage={setPage} />
        </>
      )}
    </section>
  );
}

function why(t: Translate, r: Row): string {
  const key = r.why ? whyKey(r.why) : undefined;
  return key ? t(key) : "";
}

const tone: Partial<Record<RowStatus, string>> = {
  [RowStatus.Added]: "badge-ok",
  [RowStatus.ToAdd]: "badge-accent",
  [RowStatus.KeptYours]: "badge-ok",
  [RowStatus.Removed]: "badge-warn",
  [RowStatus.ToRemove]: "badge-warn",
  [RowStatus.NoMatch]: "badge-danger",
  [RowStatus.LeftOut]: "badge-danger",
  [RowStatus.Waiting]: "badge-warn",
  [RowStatus.NotSearchedFailed]: "badge-danger",
};

export function StatusBadge({ status }: { status: RowStatus }) {
  const { t } = useT();
  if (status === RowStatus.$zero) return null;
  return <span className={`badge ${tone[status] ?? ""}`}>{t(statusKey[status])}</span>;
}

/** SpotifyPlan is the Spotify view of the plan: each song and what will
 * happen to it, linked back to Spotify, with no YouTube content. */
function SpotifyPlan({ flow, plan, onBack }: { flow: Flow; plan: Models.RunView; onBack: () => void }) {
  const { t } = useT();
  return (
    <>
      {(plan.playlists ?? []).map((p) => <SongTable key={p.sourceId} playlist={p} />)}
      <div className="actionbar">
        <span className="small muted">{t("writeFromYouTube")}</span>
        <span className="spacer" />
        <button type="button" className="btn btn-primary btn-lg" onClick={onBack} disabled={flow.busy !== null}>
          {t("goToYouTubeView")}
        </button>
      </div>
    </>
  );
}

function SongTable({ playlist }: { playlist: Models.PlaylistRun }) {
  const { t } = useT();
  const [page, setPage] = useState(0);
  const rows = (playlist.rows ?? []).filter((r) => r.position);
  return (
    <section className="panel" style={{ marginTop: 16 }} aria-label={playlist.sourceName}>
      <div className="panel-pad row" style={{ paddingBottom: 8 }}>
        <h3 title={playlist.sourceName}>{playlist.sourceName}</h3>
        <span className="spacer" />
        <ExternalLink href={playlist.sourceUrl} className="open-spotify" label={t("openSpotifyAria", { name: playlist.sourceName })}>{t("openSpotify")}</ExternalLink>
      </div>
      <div className="table-wrap">
        <table>
          <thead><tr><th className="w-pos">{t("colNumber")}</th><th className="w-main">{t("colSong")}</th><th className="w-status">{t("colStatus")}</th><th /></tr></thead>
          <tbody>
            {pageOf(rows, page).map((r) => (
              <tr key={r.position}>
                <td className="pos num">{r.position}</td>
                <td>
                  {r.spotify_track
                    ? <><span className="clip" title={r.spotify_track}>{r.spotify_track}</span><span className="sub">{r.spotify_artists}</span></>
                    : <span className="muted">—</span>}
                </td>
                <td><StatusBadge status={r.status} /></td>
                <td>{r.spotify_track_url && <ExternalLink href={r.spotify_track_url} className="open-spotify" label={t("openSpotifyAria", { name: r.spotify_track ?? "" })}>{t("openSpotify")}</ExternalLink>}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      <Pager page={page} total={rows.length} onPage={setPage}
        after={<ExternalLink href="https://open.spotify.com/" className="open-spotify">{t("openSpotify")}</ExternalLink>} />
    </section>
  );
}
