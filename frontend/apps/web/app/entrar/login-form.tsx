"use client";

import { useRouter } from "next/navigation";
import { useState } from "react";
import { ApiError } from "@mercado/api-client";
import { browserApi } from "@/lib/api-browser";

export function LoginForm() {
  const router = useRouter();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [pending, setPending] = useState(false);

  async function onSubmit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setPending(true);
    setError(null);
    try {
      await browserApi().login({ email, password });
      router.push("/");
      router.refresh();
    } catch (cause) {
      if (cause instanceof ApiError && cause.status === 401) {
        setError("E-mail ou senha inválidos.");
      } else {
        setError("Não foi possível entrar. Tente novamente.");
      }
    } finally {
      setPending(false);
    }
  }

  return (
    <form className="auth-form" onSubmit={onSubmit} data-testid="login-form">
      <div className="filter-field">
        <label htmlFor="login-email">E-mail</label>
        <input
          id="login-email"
          type="email"
          autoComplete="email"
          required
          value={email}
          onChange={(event) => setEmail(event.target.value)}
        />
      </div>
      <div className="filter-field">
        <label htmlFor="login-password">Senha</label>
        <input
          id="login-password"
          type="password"
          autoComplete="current-password"
          required
          value={password}
          onChange={(event) => setPassword(event.target.value)}
        />
      </div>
      {error ? (
        <p className="form-error" role="alert" data-testid="form-error">
          {error}
        </p>
      ) : null}
      <button type="submit" className="primary-action" disabled={pending}>
        {pending ? "Entrando..." : "Entrar"}
      </button>
      <p className="form-hint">
        Ainda não tem conta? <a href="/cadastro">Cadastre-se</a>
      </p>
    </form>
  );
}
