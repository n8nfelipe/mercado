import { createClient } from "@mercado/api-client";

export const apiBaseUrl = process.env.API_BASE_URL ?? "http://127.0.0.1:8080";

export function serverApi(cookieHeader?: string) {
  return createClient({
    baseUrl: apiBaseUrl,
    fetch: (input, init) =>
      fetch(input, {
        ...init,
        cache: "no-store",
        headers: {
          ...(init?.headers ?? {}),
          ...(cookieHeader ? { cookie: cookieHeader } : {}),
        },
      }),
  });
}
