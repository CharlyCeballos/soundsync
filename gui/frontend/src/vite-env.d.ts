/// <reference types="vite/client" />

// The popover API (WebView2 and WebKitGTK both ship it) is newer than the
// React 18 typings; React passes these lowercase attributes through as is.
declare module "react" {
  interface HTMLAttributes<T> {
    popover?: "auto" | "manual" | "";
  }
  interface ButtonHTMLAttributes<T> {
    popovertarget?: string;
    popovertargetaction?: "toggle" | "show" | "hide";
  }
}

export {};
