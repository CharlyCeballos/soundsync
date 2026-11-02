import { useEffect, useMemo, useState } from "react";
import type { Flow } from "../flow";
import { useT } from "../i18n";
import { Icon } from "../components/Icon";
import { Banner, ExternalLink, Pager, SpotifyAttribution, pageOf } from "../components/ui";
import { Connect } from "./Connect";

const SPOTIFY_HOME = "https://open.spotify.com/";

/** Choose is step 1, the Spotify view: the user's library, to pick from. */
export function Choose({ flow }: { flow: Flow }) {
  const { t } = useT();
  const account = flow.status?.spotify;
  const [query, setQuery] = useState("");
  const [page, setPage] = useState(0);

  useEffect(() => {
    if (account && flow.playlists === null && flow.busy === null && !flow.error) void flow.loadPlaylists(false);
  }, [account, flow]);

  const visible = useMemo(() => {
    const q = query.trim().toLocaleLowerCase();
    const all = flow.playlists ?? [];
    return q ? all.filter((p) => p.name.toLocaleLowerCase().includes(q)) : all;
  }, [flow.playlists, query]);
  useEffect(() => setPage(0), [query]);

  if (!account) return <div className="page"><Connect flow={flow} provider="spotify" /></div>;

  const chosen = flow.selected.length;
  return (
    <div className="page">
      <div className="attribution">
        <SpotifyAttribution />
        <div className="row small">
          <span className="muted">{t("connectedAs", { name: account.name })}</span>
          <button type="button" className="btn btn-ghost" onClick={() => void flow.disconnectSpotify()} disabled={flow.busy !== null}>
            {t("disconnect")}
          </button>
        </div>
      </div>

      <div className="page-head">
        <h1>{t("chooseTitle")}</h1>
      </div>

      <div className="row" style={{ marginBottom: 16 }}>
        <div className="search">
          <Icon name="search" />
          <label className="visually-hidden" htmlFor="filter">{t("filterLabel")}</label>
          <input id="filter" className="input input-search" type="search" placeholder={t("filterPlaceholder")} value={query} onChange={(e) => setQuery(e.target.value)} />
        </div>
        <span className="spacer" />
        <button type="button" className="btn btn-secondary" onClick={() => void flow.loadPlaylists(true)} disabled={flow.busy !== null}>
          <Icon name="refresh" />
          {t("refreshList")}
        </button>
      </div>

      {flow.error && <Banner tone="error">{flow.error}</Banner>}

      <div className="panel" aria-busy={flow.busy === "playlists"}>
        {flow.playlists === null ? (
          <ul className="list" aria-label={t("loadingPlaylists")}>
            {Array.from({ length: 6 }, (_, i) => (
              <li key={i}><div className="skeleton" style={{ width: `${40 + ((i * 17) % 35)}%` }} /></li>
            ))}
          </ul>
        ) : flow.playlists.length === 0 ? (
          <div className="panel-pad"><p className="muted">{t("emptyLibrary")}</p></div>
        ) : visible.length === 0 ? (
          <div className="panel-pad row-wrap">
            <p className="muted">{t("noFilterMatch", { query })}</p>
            <button type="button" className="btn btn-ghost" onClick={() => setQuery("")}>{t("clearFilter")}</button>
          </div>
        ) : (
          <>
            <ul className="list">
              {pageOf(visible, page).map((p) => {
                const followed = p.access === "followed";
                const on = flow.selected.includes(p.id);
                return (
                  <li key={p.id} className={`${followed ? "disabled" : ""} ${on ? "selected" : ""}`}>
                    <label className="pick">
                      <input type="checkbox" checked={on} disabled={followed} onChange={() => flow.toggle(p.id)} />
                      <span className="item-main">
                        <span className="item-title" title={p.name}>{p.name}</span>
                        <span className="item-sub">
                          {followed
                            ? t("followedNote", { owner: p.owner })
                            : <>
                                {p.tracks >= 0 && <span className="num">{t("songs", { count: p.tracks })}</span>}
                                {p.access === "collaborative" && <> · {t("collaborative")}</>}
                              </>}
                        </span>
                      </span>
                    </label>
                    <ExternalLink href={p.url} className="open-spotify" label={t("openSpotifyAria", { name: p.name })}>
                      {t("openSpotify")}
                    </ExternalLink>
                  </li>
                );
              })}
            </ul>
            <Pager page={page} total={visible.length} onPage={setPage}
              after={<ExternalLink href={SPOTIFY_HOME} className="open-spotify">{t("openSpotify")}</ExternalLink>} />
          </>
        )}
      </div>

      <div className="actionbar">
        <span className="num" aria-live="polite">{chosen > 0 ? t("selectedCount", { count: chosen }) : <span className="muted">{t("chooseHint")}</span>}</span>
        <span className="spacer" />
        <button type="button" className="btn btn-primary btn-lg" disabled={chosen === 0 || flow.busy !== null} onClick={() => void flow.review()}>
          {t("continue")}
          <Icon name="arrowRight" />
        </button>
      </div>
    </div>
  );
}
