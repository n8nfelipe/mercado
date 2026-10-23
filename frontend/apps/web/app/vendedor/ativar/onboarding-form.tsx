"use client";

import { useRouter } from "next/navigation";
import { useState } from "react";
import { ApiError } from "@mercado/api-client";
import { browserApi } from "@/lib/api-browser";

export function OnboardingForm() {
  const router = useRouter();
  const [displayName, setDisplayName] = useState("");
  const [document, setDocument] = useState("");
  const [contact, setContact] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [pending, setPending] = useState(false);

  async function onSubmit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setPending(true);
    setError(null);
    try {
      await browserApi().becomeSeller({ display_name: displayName, document, contact });
      router.push("/vendedor");
      router.refresh();
    } catch (cause) {
      if (cause instanceof ApiError && cause.status === 409) {
        setError("Este documento já está cadastrado.");
      } else if (cause instanceof ApiError && cause.status === 400) {
        setError("Confira os dados informados.");
      } else {
        setError("Não foi possível ativar a conta de vendedor.");
      }
    } finally {
      setPending(false);
    }
  }

  return (
    <form className="auth-form" onSubmit={onSubmit} data-testid="onboarding-form">
      <div className="filter-field">
        <label htmlFor="display-name">Nome da loja</label>
        <input id="display-name" required minLength={2} value={displayName} onChange={(event) => setDisplayName(event.target.value)} />
      </div>
      <div className="filter-field">
        <label htmlFor="document">Documento</label>
        <input id="document" required value={document} onChange={(event) => setDocument(event.target.value)} />
      </div>
      <div className="filter-field">
        <label htmlFor="contact">Contato</label>
        <input id="contact" required value={contact} onChange={(event) => setContact(event.target.value)} />
      </div>
      {error ? (
        <p className="form-error" role="alert" data-testid="form-error">
          {error}
        </p>
      ) : null}
      <button type="submit" className="primary-action" disabled={pending}>
        {pending ? "Ativando..." : "Começar a vender"}
      </button>
    </form>
  );
}
