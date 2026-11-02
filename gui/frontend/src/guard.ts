import { Settings } from "./api";

/**
 * installGuard keeps the window on Soundsync's own page. A link is handed to
 * the backend, which opens it in the system browser if it is on its
 * allowlist; dropping a file never navigates the window. Without this a
 * click on a link would load that page inside the app.
 */
export function installGuard(): void {
  const onLink = (event: MouseEvent) => {
    const anchor = (event.target as Element | null)?.closest?.("a[href]") as HTMLAnchorElement | null;
    if (!anchor) return;
    event.preventDefault();
    if (event.type === "click" || event.button === 1) {
      Settings.OpenURL(anchor.href).catch((err: unknown) => console.warn("link not opened:", err));
    }
  };
  document.addEventListener("click", onLink, true);
  document.addEventListener("auxclick", onLink, true);
  for (const type of ["dragover", "drop"]) {
    window.addEventListener(type, (e) => e.preventDefault());
  }
}
