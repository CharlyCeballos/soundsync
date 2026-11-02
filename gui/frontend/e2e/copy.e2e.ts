import fs from "node:fs";
import { test, expect, english, chooseAndReview } from "./fixtures";

const spotifyLogo = 'img[alt="Spotify"]:visible';
const youtubeMark = 'img[alt="Developed with YouTube"]:visible';

test("copies two playlists, keeping each brand in its own view", async ({ page, app }) => {
  await english(page);

  await page.getByRole("button", { name: "Connect Spotify" }).click();
  await expect(page.getByText("Waiting for you to log in in your browser")).toBeVisible();
  await expect(page.getByRole("link", { name: "Open the login page" })).toBeVisible();
  await expect(page.getByRole("heading", { name: "Choose the playlists to copy" })).toBeVisible();
  await expect(page.getByRole("checkbox", { name: /Pentatonix/ })).toBeDisabled();
  await expect(page.getByRole("button", { name: "Continue" })).toBeDisabled();
  await expect(page.locator(youtubeMark)).toHaveCount(0);

  // OPEN SPOTIFY leaves through the system browser; the window stays put.
  const before = page.url();
  await page.getByRole("link", { name: "Open Covers in Spotify" }).click();
  expect((await app.state()).opened).toContain("https://open.spotify.com/playlist/3yQi6hff0Ic45VJfJoaXIn");
  expect(page.url()).toBe(before);

  await page.getByRole("checkbox", { name: /The Way to My Favorite Part/ }).check();
  await page.getByRole("checkbox", { name: /Covers/ }).check();
  await expect(page.getByText("2 playlists chosen")).toBeVisible();
  await page.getByRole("button", { name: "Continue" }).click();
  await page.getByRole("button", { name: "Connect YouTube" }).click();

  await expect(page.getByText("Already on YouTube")).toHaveCount(2);
  await expect(page.locator(spotifyLogo)).toHaveCount(0);
  await page.getByRole("button", { name: "Find the songs on YouTube" }).click();

  await expect(page.getByRole("heading", { name: "What will change on Carlos Ceballos" })).toBeVisible();
  await expect(page.getByText("Add 2 videos")).toHaveCount(2);
  expect((await app.state()).counts.ItemInserts).toBe(0); // nothing written before the yes

  await page.getByRole("button", { name: "On Spotify" }).click();
  await expect(page.locator(youtubeMark)).toHaveCount(0);
  await expect(page.locator(spotifyLogo)).toHaveCount(1);
  await expect(page.getByText("Podcast episode: can't be searched")).toBeVisible();
  await expect(page.getByRole("button", { name: "Write to YouTube" })).toHaveCount(0);
  await page.getByRole("button", { name: "Go to the YouTube view to write" }).click();

  await page.getByRole("button", { name: "Write to YouTube" }).click();
  await expect(page.getByRole("heading", { name: "Your playlists are on YouTube" })).toBeVisible();
  await expect(page.getByText("Up to date")).toHaveCount(2);
  expect((await app.state()).counts.ItemInserts).toBe(4);

  await page.getByRole("button", { name: "Download as CSV (spreadsheet)" }).click();
  const saved = page.getByText(/^Saved to /);
  await expect(saved).toBeVisible();
  const reportPath = (await saved.innerText()).replace(/^Saved to /, "").trim();
  const csv = fs.readFileSync(reportPath, "utf8");
  expect(csv.split("\n")[0]).toBe("playlist,position,spotify_artists,spotify_track,spotify_track_url,status,youtube_title,youtube_channel,youtube_video_url,why,attribution");
  expect(csv).toContain("added");

  await page.getByRole("button", { name: "Copy more playlists" }).click();
  await expect(page.getByRole("heading", { name: "Choose the playlists to copy" })).toBeVisible();
  await expect(page.getByText("Choose at least one playlist.")).toBeVisible();
});

// Found in the real-account test: disconnecting showed "connect Spotify first".
test("disconnecting goes back to a clean connect screen", async ({ page }) => {
  await english(page);
  await chooseAndReview(page, ["Covers"]);
  await expect(page.getByText("Already on YouTube")).toBeVisible();
  await page.getByRole("button", { name: "Disconnect" }).click();
  await expect(page.getByRole("button", { name: "Connect YouTube" })).toBeVisible();
  await expect(page.getByRole("alert")).toHaveCount(0);

  await page.getByRole("button", { name: /Choose playlists/ }).click();
  await page.getByRole("button", { name: "Disconnect" }).click();
  await expect(page.getByRole("heading", { name: "Connect your Spotify account" })).toBeVisible();
  await expect(page.getByRole("alert")).toHaveCount(0);
});

test("a second run finds everything up to date", async ({ page }) => {
  await english(page);
  await chooseAndReview(page, ["Covers"]);
  await page.getByRole("button", { name: "Find the songs on YouTube" }).click();
  await page.getByRole("button", { name: "Write to YouTube" }).click();
  await expect(page.getByText("Up to date")).toBeVisible();

  await page.getByRole("button", { name: "Copy more playlists" }).click();
  await page.getByRole("checkbox", { name: /Covers/ }).check();
  await page.getByRole("button", { name: "Continue" }).click();
  await page.getByRole("button", { name: "Find the songs on YouTube" }).click();
  await expect(page.getByText("Everything is up to date. There is nothing to write.")).toBeVisible();
});
