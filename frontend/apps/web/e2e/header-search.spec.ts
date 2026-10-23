import { expect, test } from "@playwright/test";

test("busca geral no header navega e mantém o termo", async ({ page }) => {
  await page.goto("/anuncio/01900000-0000-7000-8000-0000000000a1");
  await page.getByLabel("Pesquisar no Mercado").fill("cap");
  await page.getByRole("button", { name: "Buscar" }).click();

  await expect(page).toHaveURL(/\/\?q=cap$/);
  await expect(page.getByTestId("listing-card")).toHaveCount(1);
  await expect(page.getByTestId("listing-card")).toContainText("Capacete");
  await expect(page.getByLabel("Pesquisar no Mercado")).toHaveValue("cap");
});
