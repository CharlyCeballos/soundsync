import { devices } from "@playwright/test";
import { defineE2EConfig } from "@gometrix/e2e-kit/config";

// Options documented in ~/.e2e/kit/config.js. Soundsync has no dev server to
// share: every test starts its own e2e build of the app (fixtures.ts), so
// tests never see each other's state.
const config = defineE2EConfig({
  dir: import.meta.dirname,
  port: 34100, // unused: the fixture picks a free port per test
  // Wails' runtime warns that a browser is not the desktop window: expected here.
  ignoreErrors: [],
});

delete config.webServer;

const fixtureOptions = { ignoreErrors: [] as RegExp[] };
const chrome = devices["Desktop Chrome"]; // WebView2 is Chromium; WebKitGTK is checked by hand
config.projects = [
  { name: "desktop-light", use: { ...chrome, ...fixtureOptions, viewport: { width: 1180, height: 800 }, colorScheme: "light" } },
  { name: "desktop-dark", use: { ...chrome, ...fixtureOptions, viewport: { width: 1180, height: 800 }, colorScheme: "dark" } },
  // The window's minimum size (gui/main.go).
  { name: "min-window", use: { ...chrome, ...fixtureOptions, viewport: { width: 900, height: 640 }, colorScheme: "light" } },
];

export default config;
