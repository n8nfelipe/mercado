import { expect, test } from "@playwright/test";

test("filtro de categoria altera a URL e o resultado", async ({ page }) => {
  await page.goto("/");
  await expect(page.getByTestId("listing-card")).toHaveCount(3);

  await page.getByLabel("Categoria").selectOption({ label: "Acessórios" });
  await page.getByRole("button", { name: "Filtrar" }).click();

  await expect(page).toHaveURL(/category_id=01900000-0000-7000-8000-0000000000c2/);
  await expect(page.getByTestId("listing-card")).toHaveCount(1);
  await expect(page.getByTestId("listing-card")).toContainText("Capacete Novo");

  await page.reload();
  await expect(page.getByTestId("listing-card")).toHaveCount(1);
  await expect(page.getByLabel("Categoria")).toHaveValue("01900000-0000-7000-8000-0000000000c2");
});

test("busca textual reflete na URL e mostra vazio", async ({ page }) => {
  await page.goto("/");
  await page.getByLabel("Busca").fill("infantil");
  await page.getByRole("button", { name: "Filtrar" }).click();

  await expect(page).toHaveURL(/q=infantil/);
  await expect(page.getByTestId("listing-card")).toHaveCount(1);
  await expect(page.getByTestId("listing-card")).toContainText("Bicicleta Infantil");

  await page.getByLabel("Busca").fill("inexistente");
  await page.getByRole("button", { name: "Filtrar" }).click();
  await expect(page.getByTestId("empty-state")).toBeVisible();
});
