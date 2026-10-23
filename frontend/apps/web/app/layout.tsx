import type { Metadata } from "next";
import { cookies } from "next/headers";
import { Suspense } from "react";
import type { User } from "@mercado/api-client";
import { SiteHeader } from "@mercado/ui";
import { serverApi } from "@/lib/api-server";
import { GlobalSearch } from "./global-search";
import { SessionMenu } from "./session-menu";
import "./globals.css";

export const metadata: Metadata = {
  title: "Mercado",
  description: "Marketplace local: compre e venda perto de você",
};

export default async function RootLayout({
  children,
}: Readonly<{ children: React.ReactNode }>) {
  let user: User | null = null;
  try {
    const cookieHeader = (await cookies()).toString();
    user = await serverApi(cookieHeader).getMe();
  } catch {
    user = null;
  }

  return (
    <html lang="pt-BR">
      <body>
        <div className="ambient" aria-hidden="true" />
        <SiteHeader
          search={
            <Suspense fallback={null}>
              <GlobalSearch />
            </Suspense>
          }
          right={<SessionMenu user={user} />}
        />
        <main className="site-main">{children}</main>
        <footer className="site-footer">
          <span>Mercado local</span>
          <span aria-hidden="true">/</span>
          <span>PostgreSQL + Redis + MinIO</span>
        </footer>
      </body>
    </html>
  );
}
