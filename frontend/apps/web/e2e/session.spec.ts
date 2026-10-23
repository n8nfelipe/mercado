import { expect, test } from "@playwright/test";

test("proxy encaminha cookie de sessão pela origem do web", async ({ request }) => {
  const anonymous = await request.get("/api/v1/auth/me");
  expect(anonymous.status()).toBe(401);

  const login = await request.post("/api/v1/auth/login", {
    data: { email: "comprador@example.com", password: "senha-super-segura" },
  });
  expect(login.status()).toBe(200);

  const me = await request.get("/api/v1/auth/me");
  expect(me.status()).toBe(200);
  const user = await me.json();
  expect(user.email).toBe("comprador@example.com");
});
