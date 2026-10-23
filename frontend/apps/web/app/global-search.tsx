"use client";

import { useRouter, useSearchParams } from "next/navigation";
import { type FormEvent, useState } from "react";

export function GlobalSearch() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const [term, setTerm] = useState(searchParams.get("q") ?? "");

  function onSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const value = term.trim();
    router.push(value ? `/?q=${encodeURIComponent(value)}` : "/");
  }

  return (
    <form className="global-search" role="search" action="/" method="get" onSubmit={onSubmit}>
      <label className="sr-only" htmlFor="global-search-q">
        Pesquisar no Mercado
      </label>
      <input
        id="global-search-q"
        name="q"
        type="search"
        value={term}
        onChange={(event) => setTerm(event.target.value)}
        placeholder="Pesquisar no Mercado"
        autoComplete="off"
      />
      <button type="submit">Buscar</button>
    </form>
  );
}
