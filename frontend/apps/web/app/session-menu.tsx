"use client";

import { useRouter } from "next/navigation";
import { useState } from "react";
import type { User } from "@mercado/api-client";
import { browserApi } from "@/lib/api-browser";

export function SessionMenu({ user }: { user: User | null }) {
  const router = useRouter();
  const [pending, setPending] = useState(false);

  if (!user) {
    return (
      <div className="session-links">
        <a href="/entrar">Entrar</a>
        <a className="session-cta" href="/cadastro">
          Criar conta
        </a>
      </div>
    );
  }

  async function logout() {
    setPending(true);
    try {
      await browserApi().logout();
      router.push("/");
      router.refresh();
    } finally {
      setPending(false);
    }
  }

  return (
    <div className="session-links">
      <span className="session-name" data-testid="session-name">
        {user.name}
      </span>
      <button type="button" onClick={logout} disabled={pending} data-testid="logout-button">
        Sair
      </button>
    </div>
  );
}
