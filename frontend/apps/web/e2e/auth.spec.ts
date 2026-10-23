import { expect, test } from "@playwright/test";

test.describe.configure({ mode: "serial" });

test("cadastro entra automaticamente e mantém a sessão", async ({ page }) => {
  const email = `comprador-${Date.now()}@example.com`;
  await page.goto("/cadastro");

  await page.getByLabel("Nome").fill("Comprador Teste");
  await page.getByLabel("E-mail").fill(email);
  await page.getByLabel("Senha").fill("senha-super-segura");
  await page.getByRole("button", { name: "Criar conta" }).click();

  await expect(page).toHaveURL("/");
  await expect(page.getByTestId("session-name")).toBeVisible();

  await page.reload();
  await expect(page.getByTestId("session-name")).toBeVisible();
});

test("logout encerra a sessão", async ({ page }) => {
  await page.goto("/entrar");
  await page.getByLabel("E-mail").fill("comprador@example.com");
  await page.getByLabel("Senha").fill("senha-super-segura");
  await page.getByRole("button", { name: "Entrar" }).click();

  await expect(page.getByTestId("session-name")).toBeVisible();
  await page.getByTestId("logout-button").click();
  await expect(page.getByRole("link", { name: "Entrar" })).toBeVisible();
});

test("mostra erro para credenciais inválidas", async ({ page }) => {
  await page.goto("/entrar");
  await page.getByLabel("E-mail").fill("erro@example.com");
  await page.getByLabel("Senha").fill("senha-super-segura");
  await page.getByRole("button", { name: "Entrar" }).click();

  await expect(page.getByTestId("form-error")).toHaveText("E-mail ou senha inválidos.");
});
