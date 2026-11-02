import { useState } from "react";
import { Clipboard } from "@wailsio/runtime";
import type { Flow } from "../flow";
import { useT } from "../i18n";
import { Icon, Spinner } from "../components/Icon";
import { Banner, ExternalLink, SpotifyAttribution } from "../components/ui";

/** Connect asks for one service's browser login and waits for it, with a
 * way out when the browser does not come up. */
export function Connect({ flow, provider }: { flow: Flow; provider: "spotify" | "youtube" }) {
  const { t } = useT();
  const [copied, setCopied] = useState(false);
  const waiting = flow.busy === provider;
  const loginURL = flow.login?.provider === provider ? flow.login.url : null;
  const isSpotify = provider === "spotify";

  const copy = async () => {
    if (!loginURL) return;
    await Clipboard.SetText(loginURL);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  return (
    <div className="center-panel">
      {isSpotify && <SpotifyAttribution />}
      <h2>{t(isSpotify ? "connectSpotifyTitle" : "connectYouTubeTitle")}</h2>
      <p>{t(isSpotify ? "connectSpotifyBody" : "connectYouTubeBody")}</p>

      {waiting ? (
        <div className="waiting" aria-live="polite">
          <div className="row">
            <Spinner />
            <span>{t("waitingLogin")}</span>
          </div>
          {loginURL && (
            <div className="row-wrap small" style={{ justifyContent: "center" }}>
              <span className="muted">{t("browserDidNotOpen")}</span>
              <ExternalLink href={loginURL}>{t("openLoginPage")}</ExternalLink>
              <button type="button" className="btn btn-ghost" onClick={() => void copy()}>
                <Icon name={copied ? "check" : "copy"} />
                {copied ? t("copied") : t("copyLink")}
              </button>
            </div>
          )}
          <button type="button" className="btn btn-secondary" onClick={flow.cancel}>
            {t("cancel")}
          </button>
        </div>
      ) : (
        <button
          type="button"
          className="btn btn-primary btn-lg"
          onClick={() => void (isSpotify ? flow.connectSpotify() : flow.connectYouTube())}
          disabled={flow.busy !== null}
        >
          {t(isSpotify ? "connectSpotify" : "connectYouTube")}
          <Icon name="external" />
        </button>
      )}
      {flow.error && !waiting && <Banner tone="error">{flow.error}</Banner>}
    </div>
  );
}
