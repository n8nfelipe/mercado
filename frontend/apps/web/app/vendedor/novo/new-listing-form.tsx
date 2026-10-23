"use client";

import { useRouter } from "next/navigation";
import { useState } from "react";
import type { Category } from "@mercado/api-client";
import { ApiError } from "@mercado/api-client";
import { browserApi } from "@/lib/api-browser";

export function NewListingForm({ categories }: { categories: Category[] }) {
  const router = useRouter();
  const [title, setTitle] = useState("");
  const [description, setDescription] = useState("");
  const [categoryId, setCategoryId] = useState(categories[0]?.id ?? "");
  const [price, setPrice] = useState("");
  const [condition, setCondition] = useState<"new" | "used">("used");
  const [freeShipping, setFreeShipping] = useState(false);
  const [stock, setStock] = useState("1");
  const [error, setError] = useState<string | null>(null);
  const [pending, setPending] = useState(false);

  async function onSubmit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setPending(true);
    setError(null);
    try {
      const listing = await browserApi().createListing({
        title,
        description,
        category_id: categoryId,
        price_cents: Math.round(Number(price) * 100),
        currency: "BRL",
        condition,
        free_shipping: freeShipping,
        stock: Number(stock),
      });
      router.push(`/vendedor/${listing.id}`);
      router.refresh();
    } catch (cause) {
      if (cause instanceof ApiError && cause.status === 400) {
        setError("Confira os campos: título, descrição, categoria, preço e estoque.");
      } else {
        setError("Não foi possível criar o anúncio.");
      }
    } finally {
      setPending(false);
    }
  }

  return (
    <form className="listing-form" onSubmit={onSubmit} data-testid="new-listing-form">
      <div className="filter-field">
        <label htmlFor="title">Título</label>
        <input id="title" required minLength={10} maxLength={120} value={title} onChange={(event) => setTitle(event.target.value)} />
      </div>
      <div className="filter-field">
        <label htmlFor="description">Descrição</label>
        <textarea id="description" required minLength={20} maxLength={5000} rows={5} value={description} onChange={(event) => setDescription(event.target.value)} />
      </div>
      <div className="filter-field">
        <label htmlFor="category">Categoria</label>
        <select id="category" required value={categoryId} onChange={(event) => setCategoryId(event.target.value)}>
          {categories.map((category) => (
            <option key={category.id} value={category.id}>
              {category.name}
            </option>
          ))}
        </select>
      </div>
      <div className="filter-field">
        <label htmlFor="price">Preço (R$)</label>
        <input id="price" type="number" min="0.01" step="0.01" required value={price} onChange={(event) => setPrice(event.target.value)} />
      </div>
      <div className="filter-field">
        <label htmlFor="condition">Condição</label>
        <select id="condition" value={condition} onChange={(event) => setCondition(event.target.value as "new" | "used")}>
          <option value="used">Usado</option>
          <option value="new">Novo</option>
        </select>
      </div>
      <div className="filter-field">
        <label htmlFor="stock">Estoque</label>
        <input id="stock" type="number" min="0" required value={stock} onChange={(event) => setStock(event.target.value)} />
      </div>
      <label className="filter-check">
        <input type="checkbox" checked={freeShipping} onChange={(event) => setFreeShipping(event.target.checked)} />
        Frete grátis
      </label>
      {error ? (
        <p className="form-error" role="alert" data-testid="form-error">
          {error}
        </p>
      ) : null}
      <button type="submit" className="primary-action" disabled={pending}>
        {pending ? "Criando..." : "Criar anúncio"}
      </button>
    </form>
  );
}
