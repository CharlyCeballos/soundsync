import { useState } from "react";
import type { Flow } from "../flow";
import { useT } from "../i18n";
import { Icon } from "../components/Icon";
import { Banner, ExternalLink } from "../components/ui";

/** Privacy is the consent gate before any login (YouTube ToS §7,
 * Developer Policies III.A.1–A.2): what happens, then the links. */
export function Privacy({ flow }: { flow: Flow }) {
  const { t } = useT();
  const [declined, setDeclined] = useState(false);
  const links = flow.status!.links;
  return (
    <div className="page">
      <div className="panel panel-pad stack-lg" style={{ maxWidth: 680, margin: "0 auto" }}>
        <h1>{t("privacyTitle")}</h1>
        <div className="stack">
          <p>{t("privacyWhat")}</p>
          <p>{t("privacyKeep")}</p>
          <p>{t("privacyTerms")}</p>
        </div>
        <section aria-labelledby="read" className="stack">
          <h3 id="read">{t("privacyRead")}</h3>
          <ul className="stack" style={{ gap: 6 }}>
            {[
              [links.policy, t("footerPrivacy")],
              [links.youtubeTerms, t("footerTerms")],
              [links.googlePrivacy, t("footerGooglePrivacy")],
              [links.googlePermissions, t("privacyRevoke")],
            ].map(([href, label]) => (
              <li key={href}>
                <ExternalLink href={href}>{label}</ExternalLink>
              </li>
            ))}
          </ul>
        </section>
        {declined && <Banner tone="info">{t("privacyDeclined")}</Banner>}
        {flow.error && <Banner tone="error">{flow.error}</Banner>}
        <div className="row">
          <button type="button" className="btn btn-primary btn-lg" onClick={() => void flow.acceptPolicy()}>
            <Icon name="check" />
            {t("privacyAccept")}
          </button>
          <button type="button" className="btn btn-secondary btn-lg" onClick={() => setDeclined(true)}>
            {t("privacyDecline")}
          </button>
        </div>
      </div>
    </div>
  );
}
