// Authored icons: one 24-unit grid, 1.75 stroke, round caps and joins.
const paths = {
  check: "M5 12.5l4.5 4.5L19 7.5",
  chevronLeft: "M14.5 6l-6 6 6 6",
  chevronRight: "M9.5 6l6 6-6 6",
  arrowRight: "M5 12h13M13 6.5l5.5 5.5-5.5 5.5",
  external: "M14 5h5v5M19 5l-8 8M17 13.5V18a1 1 0 0 1-1 1H6a1 1 0 0 1-1-1V8a1 1 0 0 1 1-1h4.5",
  refresh: "M19.5 12a7.5 7.5 0 1 1-2.2-5.3M19.5 4.5v4h-4",
  folder: "M3.5 7.5a1.5 1.5 0 0 1 1.5-1.5h4l2 2h8a1.5 1.5 0 0 1 1.5 1.5v8A1.5 1.5 0 0 1 19 19H5a1.5 1.5 0 0 1-1.5-1.5z",
  file: "M7 3.5h6.5l4 4V19a1.5 1.5 0 0 1-1.5 1.5H7A1.5 1.5 0 0 1 5.5 19V5A1.5 1.5 0 0 1 7 3.5zM13.5 3.5V8h4",
  info: "M12 21a9 9 0 1 0 0-18 9 9 0 0 0 0 18zM12 11v5.5M12 7.5v.5",
  alert: "M12 21a9 9 0 1 0 0-18 9 9 0 0 0 0 18zM12 7.5v5.5M12 16v.5",
  warning: "M12 4l9 15.5H3zM12 10v4M12 16.8v.2",
  x: "M6.5 6.5l11 11M17.5 6.5l-11 11",
  search: "M10.5 17.5a7 7 0 1 0 0-14 7 7 0 0 0 0 14zM15.5 15.5L20 20",
  copy: "M9 9h10v10H9zM6 15H5V5h10v1",
  download: "M12 4v11M7 10.5l5 5 5-5M5 19.5h14",
  stop: "M7 7h10v10H7z",
} as const;

export type IconName = keyof typeof paths;

export function Icon({ name, label }: { name: IconName; label?: string }) {
  return (
    <svg
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth={1.75}
      strokeLinecap="round"
      strokeLinejoin="round"
      role={label ? "img" : undefined}
      aria-label={label}
      aria-hidden={label ? undefined : true}
    >
      <path d={paths[name]} />
    </svg>
  );
}

export function Spinner({ label }: { label?: string }) {
  return (
    <svg className="spinner" viewBox="0 0 24 24" fill="none" role={label ? "img" : undefined} aria-label={label} aria-hidden={label ? undefined : true}>
      <circle cx="12" cy="12" r="9" stroke="currentColor" strokeOpacity={0.2} strokeWidth={2.5} />
      <path d="M21 12a9 9 0 0 0-9-9" stroke="currentColor" strokeWidth={2.5} strokeLinecap="round" />
    </svg>
  );
}

/** AppMark is Soundsync's own mark (gui/build/appicon.svg). */
export function AppMark() {
  return (
    <svg viewBox="0 0 1024 1024" aria-hidden>
      <defs>
        <linearGradient id="mark-bg" x1="0" y1="0" x2="1" y2="1">
          <stop offset="0" stopColor="#4338CA" />
          <stop offset="1" stopColor="#7C3AED" />
        </linearGradient>
      </defs>
      <rect x="48" y="48" width="928" height="928" rx="216" fill="url(#mark-bg)" />
      <g fill="#fff">
        <rect x="200" y="284" width="352" height="96" rx="48" />
        <rect x="200" y="644" width="352" height="96" rx="48" />
        <rect x="200" y="464" width="560" height="96" rx="48" />
      </g>
      <path d="M650 372 L790 512 L650 652" fill="none" stroke="#fff" strokeWidth={96} strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
}
