import { expect, test } from "@playwright/test";

const suspended = "01900000-0000-7000-8000-0000000000a9";

test("abre o detalhe de um anúncio publicado", async ({ page }) => {
  await page.goto("/");
  await page.getByTestId("listing-card").filter({ hasText: "Aro 29 Usada" }).click();

  await expect(page.getByTestId("listing-detail")).toBeVisible();
  await expect(page.getByTestId("detail-title")).toHaveText("Bicicleta Aro 29 Usada");
  await expect(page.getByTestId("detail-price")).toContainText("1.200,00");
  await expect(page.getByTestId("detail-stock")).toHaveText("2");
  await expect(page.getByTestId("detail-seller")).toHaveText("Loja do Bairro");
});

test("anúncio suspenso retorna não encontrado", async ({ page }) => {
  await page.goto(`/anuncio/${suspended}`);
  await expect(page.getByRole("heading", { name: "Anúncio não encontrado" })).toBeVisible();
});
