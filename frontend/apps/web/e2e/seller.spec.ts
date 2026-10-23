import { expect, test } from "@playwright/test";

test.describe.configure({ mode: "serial" });

test("fluxo completo do vendedor", async ({ page }) => {
  const email = `vendedor-${Date.now()}@example.com`;

  await page.goto("/cadastro");
  await page.getByLabel("Nome").fill("Vendedor Teste");
  await page.getByLabel("E-mail").fill(email);
  await page.getByLabel("Senha").fill("senha-super-segura");
  await page.getByRole("button", { name: "Criar conta" }).click();
  await expect(page).toHaveURL("/");

  // Comprador não acessa a área do vendedor.
  await page.goto("/vendedor");
  await expect(page).toHaveURL(/\/vendedor\/ativar/);

  // Ativa a conta de vendedor.
  await page.getByLabel("Nome da loja").fill("Loja E2E");
  await page.getByLabel("Documento").fill("12345678901");
  await page.getByLabel("Contato").fill(email);
  await page.getByRole("button", { name: "Começar a vender" }).click();
  await expect(page).toHaveURL("/vendedor");
  await expect(page.getByTestId("empty-state")).toBeVisible();

  // Cria um anúncio.
  await page.getByRole("link", { name: "Novo anúncio" }).click();
  await page.getByLabel("Título").fill("Vinil Clássico em Ótimo Estado");
  await page.getByLabel("Descrição").fill("Vinil raro, capa e disco em ótimo estado de conservação.");
  await page.getByLabel("Preço (R$)").fill("150");
  await page.getByLabel("Estoque").fill("2");
  await page.getByRole("button", { name: "Criar anúncio" }).click();

  await expect(page.getByTestId("editor-status")).toHaveText("Rascunho");
  const listingId = page.url().split("/").pop()!;

  // Envia uma imagem e confirma que ela vira capa.
  await page.getByTestId("image-input").setInputFiles({
    name: "capa.png",
    mimeType: "image/png",
    buffer: Buffer.from("89504e470d0a1a0a", "hex"),
  });
  await expect(page.getByTestId("image-item")).toHaveCount(1);
  await expect(page.getByTestId("image-item")).toContainText("Capa");

  // Envia para revisão.
  await page.getByTestId("submit-listing").click();
  await expect(page.getByTestId("editor-status")).toHaveText("Em revisão");

  // Rejeição mostra o motivo e permite reenviar.
  await page.request.post(`/api/v1/admin/listings/${listingId}/reject`, { data: { reason: "Fotos insuficientes" } });
  await page.reload();
  await expect(page.getByTestId("rejection-reason")).toContainText("Fotos insuficientes");
  await page.getByTestId("submit-listing").click();
  await expect(page.getByTestId("editor-status")).toHaveText("Em revisão");

  // Aprovação publica o anúncio e ele aparece com a capa escolhida.
  await page.request.post(`/api/v1/admin/listings/${listingId}/publish`);
  await page.reload();
  await expect(page.getByTestId("editor-status")).toHaveText("Publicado");

  await page.goto(`/anuncio/${listingId}`);
  await expect(page.getByTestId("detail-title")).toHaveText("Vinil Clássico em Ótimo Estado");
  await expect(page.locator(".detail-cover")).toHaveAttribute("src", /\/v1\/media\//);
  await expect(page.getByTestId("detail-stock")).toHaveText("2");

  // Estoque zero deixa o anúncio indisponível no detalhe público.
  await page.goto(`/vendedor/${listingId}`);
  await page.getByTestId("stock-delta").fill("-2");
  await page.getByTestId("stock-reason").fill("últimas unidades vendidas");
  await page.getByTestId("adjust-stock").click();
  await expect(page.getByTestId("editor-stock")).toHaveText("estoque 0");
  await expect(page.getByTestId("stock-history")).toContainText("últimas unidades vendidas");

  await page.goto(`/anuncio/${listingId}`);
  await expect(page.getByTestId("detail-stock")).toHaveText("Indisponível");
});
