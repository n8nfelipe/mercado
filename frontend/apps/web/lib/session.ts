import { cookies } from "next/headers";
import { redirect } from "next/navigation";
import type { ApiClient, User } from "@mercado/api-client";
import { serverApi } from "@/lib/api-server";

export interface SellerSession {
  api: ApiClient;
  user: User;
  cookieHeader: string;
}

export async function requireSeller(): Promise<SellerSession> {
  const cookieHeader = (await cookies()).toString();
  const api = serverApi(cookieHeader);
  let user: User;
  try {
    user = await api.getMe();
  } catch {
    redirect("/entrar?next=/vendedor");
  }
  if (!user.roles.includes("seller")) {
    redirect("/vendedor/ativar");
  }
  return { api, user, cookieHeader };
}
