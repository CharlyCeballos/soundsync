import { spawn, type ChildProcess } from "node:child_process";
import fs from "node:fs";
import net from "node:net";
import path from "node:path";
import { test as base, expect } from "@gometrix/e2e-kit";

// Built by `wails3 task build:e2e` (npm run e2e:build).
const BINARY = path.join(import.meta.dirname, "..", "..", "bin", process.platform === "win32" ? "soundsync-gui-e2e.exe" : "soundsync-gui-e2e");
const START_TIMEOUT_MS = 15_000;

export type Scenario = "noconfig" | "nopolicy" | "ready" | "samename" | "quota" | "many";

/** What reached the fake world (gui/e2e.go, /__e2e/state). */
export type WorldState = {
  dir: string;
  opened: string[] | null;
  counts: { Inserts: number; Updates: number; Searches: number; ItemInserts: number; Deletes: number };
  revoked: string[] | null;
};

export type App = {
  /** state reads what the app did to the fake Spotify, YouTube and OS. */
  state(): Promise<WorldState>;
};

function freePort(): Promise<number> {
  return new Promise((resolve, reject) => {
    const srv = net.createServer();
    srv.once("error", reject);
    srv.listen(0, "127.0.0.1", () => {
      const port = (srv.address() as net.AddressInfo).port;
      srv.close(() => resolve(port));
    });
  });
}

async function waitUntilUp(url: string, proc: ChildProcess): Promise<void> {
  const deadline = Date.now() + START_TIMEOUT_MS;
  while (Date.now() < deadline) {
    if (proc.exitCode !== null) throw new Error(`the e2e app exited with code ${proc.exitCode}`);
    try {
      const res = await fetch(url);
      if (res.ok) return;
    } catch {
      // not listening yet
    }
    await new Promise((r) => setTimeout(r, 100));
  }
  throw new Error(`the e2e app did not answer on ${url}`);
}

export const test = base.extend<{ scenario: Scenario; app: App }>({
  scenario: ["ready", { option: true }],

  /** app starts this test's own copy of the app and opens it (every test). */
  app: [async ({ page, scenario }, use) => {
    if (!fs.existsSync(BINARY)) throw new Error(`missing ${BINARY}: run \`npm run e2e:build\` first`);
    const port = await freePort();
    const proc = spawn(BINARY, ["-scenario", scenario], {
      env: { ...process.env, WAILS_SERVER_PORT: String(port) },
      stdio: "ignore",
    });
    const base = `http://localhost:${port}`;
    const state = async () => (await (await fetch(base + "/__e2e/state")).json()) as WorldState;
    let dir = "";
    try {
      await waitUntilUp(base + "/", proc);
      dir = (await state()).dir;
      await page.goto(base + "/");
      await use({ state });
    } finally {
      proc.kill();
      if (dir.includes("soundsync-e2e-")) fs.rmSync(dir, { recursive: true, force: true });
    }
  }, { auto: true }],
});

/** english makes the page deterministic whatever the system language. */
export async function english(page: import("@playwright/test").Page) {
  await page.getByRole("button", { name: /^EN/ }).click();
}

/** connectBoth logs in to both fake services and picks playlists by name. */
export async function chooseAndReview(page: import("@playwright/test").Page, names: string[]) {
  await page.getByRole("button", { name: "Connect Spotify" }).click();
  for (const name of names) {
    await page.getByRole("checkbox", { name: new RegExp(name) }).check();
  }
  await page.getByRole("button", { name: "Continue" }).click();
  await page.getByRole("button", { name: "Connect YouTube" }).click();
}

export { expect };
