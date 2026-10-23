import { expect, test } from "@playwright/test";

test.describe.configure({ mode: "serial" });

test("mostra a vitrine com anúncios publicados", async ({ page }) => {
  await page.goto("/");

  await expect(page.getByRole("heading", { name: "Encontre o próximo achado" })).toBeVisible();
  await expect(page.getByTestId("results-total")).toHaveText("3 anúncios");
  await expect(page.getByTestId("listing-card")).toHaveCount(3);
  await expect(page.getByTestId("listing-card").first()).toContainText("Bicicleta");
});

test("mostra indisponibilidade quando a API falha", async ({ page, request }) => {
  await request.post("http://127.0.0.1:18082/toggle");
  try {
    await page.goto("/");
    await expect(page.getByRole("heading", { name: "API indisponível" })).toBeVisible();
    await expect(page.getByTestId("status-error")).toContainText("503");
  } finally {
    await request.post("http://127.0.0.1:18082/toggle");
  }
});
