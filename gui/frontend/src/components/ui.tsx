import type { ReactNode } from "react";
import { Icon, type IconName } from "./Icon";
import { useT } from "../i18n";
import spotifyBlack from "../assets/brands/spotify-full-black.svg";
import spotifyWhite from "../assets/brands/spotify-full-white.svg";
import ytDarkText from "../assets/brands/developed-with-youtube-dark-text.png";
import ytLightText from "../assets/brands/developed-with-youtube-light-text.png";

type Tone = "info" | "ok" | "warn" | "error";
const toneIcon: Record<Tone, IconName> = { info: "info", ok: "check", warn: "warning", error: "alert" };

/** Banner states something about the page; errors are announced. */
export function Banner({ tone, children, action }: { tone: Tone; children: ReactNode; action?: ReactNode }) {
  return (
    <div className={`banner banner-${tone}`} role={tone === "error" ? "alert" : "status"}>
      <Icon name={toneIcon[tone]} />
      <div className="body multiline">{children}</div>
      {action}
    </div>
  );
}

/** ExternalLink opens in the system browser (see guard.ts); it never
 * navigates the window. */
export function ExternalLink({ href, children, className, label }: { href: string; children: ReactNode; className?: string; label?: string }) {
  return (
    <a href={href} className={className} aria-label={label} rel="noreferrer" target="_blank">
      {children}
    </a>
  );
}

/** SpotifyAttribution is the official full logo, black on light and white
 * on dark (green only on pure black or white), with its clear space. */
export function SpotifyAttribution() {
  return (
    <>
      <img className="spotify-logo only-light" src={spotifyBlack} alt="Spotify" />
      <img className="spotify-logo only-dark" src={spotifyWhite} alt="Spotify" />
    </>
  );
}

/** YouTubeAttribution is the "Developed with YouTube" mark, linked to the
 * channel the playlists go to (Branding Guidelines: logos link to YouTube). */
export function YouTubeAttribution({ channelURL }: { channelURL: string }) {
  return (
    <ExternalLink href={channelURL}>
      <img className="yt-badge only-light" src={ytDarkText} alt="Developed with YouTube" />
      <img className="yt-badge only-dark" src={ytLightText} alt="Developed with YouTube" />
    </ExternalLink>
  );
}

export const PAGE_SIZE = 20; // Spotify: never more than 20 items in a content set

/** Pager moves through a list in sets of PAGE_SIZE. */
export function Pager({ page, total, onPage, after }: { page: number; total: number; onPage: (p: number) => void; after?: ReactNode }) {
  const { t } = useT();
  const pages = Math.max(1, Math.ceil(total / PAGE_SIZE));
  const from = total === 0 ? 0 : page * PAGE_SIZE + 1;
  const to = Math.min(total, (page + 1) * PAGE_SIZE);
  return (
    <div className="pager">
      <span className="num">{t("pagerRange", { from, to, total })}</span>
      {after}
      {pages > 1 && (
        <div className="row">
          <button type="button" className="btn btn-secondary icon-btn" onClick={() => onPage(page - 1)} disabled={page === 0} aria-label={t("pagerPrevious")}>
            <Icon name="chevronLeft" />
          </button>
          <button type="button" className="btn btn-secondary icon-btn" onClick={() => onPage(page + 1)} disabled={page >= pages - 1} aria-label={t("pagerNext")}>
            <Icon name="chevronRight" />
          </button>
        </div>
      )}
    </div>
  );
}

export function pageOf<T>(items: T[], page: number): T[] {
  return items.slice(page * PAGE_SIZE, (page + 1) * PAGE_SIZE);
}
