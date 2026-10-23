"use client";

import { useRouter } from "next/navigation";
import { useState } from "react";
import { ApiError } from "@mercado/api-client";
import { browserApi } from "@/lib/api-browser";

export function RegisterForm() {
  const router = useRouter();
  const [name, setName] = useState("");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [pending, setPending] = useState(false);

  async function onSubmit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setPending(true);
    setError(null);
    try {
      await browserApi().register({ name, email, password });
      await browserApi().login({ email, password });
      router.push("/");
      router.refresh();
    } catch (cause) {
      if (cause instanceof ApiError && cause.status === 409) {
        setError("Já existe uma conta com este e-mail.");
      } else if (cause instanceof ApiError && cause.status === 400) {
        setError("Confira os dados: nome, e-mail válido e senha com ao menos 12 caracteres.");
      } else {
        setError("Não foi possível concluir o cadastro. Tente novamente.");
      }
    } finally {
      setPending(false);
    }
  }

  return (
    <form className="auth-form" onSubmit={onSubmit} data-testid="register-form">
      <div className="filter-field">
        <label htmlFor="register-name">Nome</label>
        <input
          id="register-name"
          type="text"
          autoComplete="name"
          required
          minLength={2}
          value={name}
          onChange={(event) => setName(event.target.value)}
        />
      </div>
      <div className="filter-field">
        <label htmlFor="register-email">E-mail</label>
        <input
          id="register-email"
          type="email"
          autoComplete="email"
          required
          value={email}
          onChange={(event) => setEmail(event.target.value)}
        />
      </div>
      <div className="filter-field">
        <label htmlFor="register-password">Senha</label>
        <input
          id="register-password"
          type="password"
          autoComplete="new-password"
          required
          minLength={12}
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
        {pending ? "Criando..." : "Criar conta"}
      </button>
      <p className="form-hint">
        Já tem conta? <a href="/entrar">Entrar</a>
      </p>
    </form>
  );
}
