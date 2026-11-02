import fs from "node:fs";
import path from "node:path";
import { test, expect, english } from "./fixtures";

const filledEnv =
  "SPOTIFY_CLIENT_ID=0123456789abcdef0123456789abcdef\n" +
  "GOOGLE_CLIENT_ID=123-abc.apps.googleusercontent.com\n" +
  "GOOGLE_CLIENT_SECRET=GOCSPX-e2e\n";

test.describe("without a settings file", () => {
  test.use({ scenario: "noconfig" });

  test("creates the file, then moves on by itself once it is filled", async ({ page, app }) => {
    await english(page);
    await expect(page.getByRole("heading", { name: "Set up your Spotify and Google apps" })).toBeVisible();
    await expect(page.getByText("Not created yet")).toBeVisible();

    await page.getByRole("button", { name: "Create the settings file" }).click();
    await expect(page.getByText("SPOTIFY_CLIENT_ID", { exact: true })).toBeVisible();
    await expect(page.getByText("Missing")).toHaveCount(3); // the two IDs and the secret
    const world = await app.state();
    expect(world.opened).toContain(world.dir);
    const created = fs.readFileSync(path.join(world.dir, ".env"), "utf8");
    expect(created).toContain("SPOTIFY_REDIRECT_URI=http://127.0.0.1:8888/callback");

    // The user fills the file in an editor and comes back to the window.
    fs.writeFileSync(path.join(world.dir, ".env"), filledEnv);
    await page.evaluate(() => window.dispatchEvent(new Event("focus")));
    await expect(page.getByRole("heading", { name: "Before you connect" })).toBeVisible();
  });
});

test.describe("before consent", () => {
  test.use({ scenario: "nopolicy" });

  test("explains the data use and waits for an explicit yes", async ({ page, app }) => {
    await english(page);
    await expect(page.getByRole("heading", { name: "Before you connect" })).toBeVisible();
    for (const name of ["Privacy policy", "YouTube Terms of Service", "Google Privacy Policy"]) {
      await expect(page.getByRole("main").getByRole("link", { name, exact: true })).toBeVisible();
    }
    await page.getByRole("button", { name: "Not now" }).click();
    await expect(page.getByText("can't connect without your agreement")).toBeVisible();

    // A link opens in the system browser, never inside the window.
    await page.getByRole("main").getByRole("link", { name: "YouTube Terms of Service", exact: true }).click();
    expect((await app.state()).opened).toContain("https://www.youtube.com/t/terms");
    await expect(page.getByRole("heading", { name: "Before you connect" })).toBeVisible();

    await page.getByRole("button", { name: "I agree, continue" }).click();
    await expect(page.getByRole("heading", { name: "Connect your Spotify account" })).toBeVisible();
  });
});
