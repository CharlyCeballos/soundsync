import { test, expect, english, chooseAndReview } from "./fixtures";

test.describe("same-title playlist (todo 4.4)", () => {
  test.use({ scenario: "samename" });

  test("uses the user's playlist after asking", async ({ page, app }) => {
    await english(page);
    await chooseAndReview(page, ["Covers"]);
    await expect(page.getByText("Which one should it use?")).toBeVisible();
    const find = page.getByRole("button", { name: "Find the songs on YouTube" });
    await expect(find).toBeDisabled();

    await page.getByRole("radio", { name: /Use “Covers”/ }).check();
    await find.click();
    await expect(page.getByText("Use “Covers” and add Soundsync's line to its description")).toBeVisible();
    await page.getByRole("button", { name: "Write to YouTube" }).click();
    await expect(page.getByRole("heading", { name: "Your playlists are on YouTube" })).toBeVisible();
    // Adopting combines: its own video stays this time, and the app says so.
    await expect(page.getByText("Its other videos were kept")).toBeVisible();
    const { counts } = await app.state();
    expect(counts.Updates).toBe(1);
    expect(counts.Inserts).toBe(0);
  });

  test("creates a new playlist when the user says so", async ({ page, app }) => {
    await english(page);
    await chooseAndReview(page, ["Covers"]);
    await page.getByRole("radio", { name: "Create a new playlist" }).check();
    await page.getByLabel("New playlists will be").selectOption("unlisted");
    await page.getByRole("button", { name: "Find the songs on YouTube" }).click();
    await expect(page.getByText("Create “Covers” (unlisted)")).toBeVisible();
    await page.getByRole("button", { name: "Write to YouTube" }).click();
    await expect(page.getByText("Up to date")).toBeVisible();
    expect((await app.state()).counts.Inserts).toBe(1);
  });
});

test.describe("a large library", () => {
  test.use({ scenario: "many" });

  test("shows 20 playlists per page and filters by name", async ({ page }) => {
    await english(page);
    await page.getByRole("button", { name: "Connect Spotify" }).click();
    await expect(page.getByText("1–20 of 25")).toBeVisible();
    await expect(page.getByRole("checkbox")).toHaveCount(20);
    await page.getByRole("button", { name: "Next page" }).click();
    await expect(page.getByText("21–25 of 25")).toBeVisible();
    await expect(page.getByRole("checkbox")).toHaveCount(5);

    await page.getByLabel("Filter playlists").fill("Mix 2");
    await expect(page.getByRole("checkbox")).toHaveCount(3); // Mix 20, 21, 22
    await page.getByLabel("Filter playlists").fill("nothing like this");
    await expect(page.getByText("No playlist matches")).toBeVisible();
    await page.getByRole("button", { name: "Clear filter" }).click();
    await expect(page.getByText("1–20 of 25")).toBeVisible();
  });
});

test.describe("daily quota", () => {
  test.use({ scenario: "quota" });

  test("stops cleanly and says why, in Spanish", async ({ page }) => {
    await page.getByRole("button", { name: /^ES/ }).click();
    await page.getByRole("button", { name: "Conectar Spotify" }).click();
    await page.getByRole("checkbox", { name: /Covers/ }).check();
    await page.getByRole("button", { name: "Continuar" }).click();
    await page.getByRole("button", { name: "Conectar YouTube" }).click();
    await page.getByRole("button", { name: "Buscar las canciones en YouTube" }).click();
    await page.getByRole("button", { name: "Escribir en YouTube" }).click();
    await expect(page.getByRole("heading", { name: "La copia se detuvo" })).toBeVisible();
    await expect(page.getByText("Se agotó la quota diaria de YouTube").first()).toBeVisible();
    await expect(page.getByText("Detenida")).toBeVisible();
  });
});
