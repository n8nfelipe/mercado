"use client";

import { useRouter } from "next/navigation";
import { useState } from "react";
import type { Category, InventoryAdjustment, ListingImage, SellerListing } from "@mercado/api-client";
import { ApiError } from "@mercado/api-client";
import { browserApi } from "@/lib/api-browser";

const statusLabels: Record<string, string> = {
  draft: "Rascunho",
  pending_review: "Em revisão",
  published: "Publicado",
  rejected: "Rejeitado",
  suspended: "Suspenso",
  archived: "Arquivado",
};

export interface ListingEditorProps {
  listing: SellerListing;
  categories: Category[];
  adjustments: InventoryAdjustment[];
}

export function ListingEditor({ listing: initialListing, categories, adjustments: initialAdjustments }: ListingEditorProps) {
  const router = useRouter();
  const [listing, setListing] = useState(initialListing);
  const [adjustments, setAdjustments] = useState(initialAdjustments);
  const [title, setTitle] = useState(initialListing.title);
  const [description, setDescription] = useState(initialListing.description);
  const [categoryId, setCategoryId] = useState(initialListing.category.id);
  const [price, setPrice] = useState((initialListing.price_cents / 100).toFixed(2));
  const [condition, setCondition] = useState<"new" | "used">(initialListing.condition);
  const [freeShipping, setFreeShipping] = useState(initialListing.free_shipping);
  const [delta, setDelta] = useState("1");
  const [reason, setReason] = useState("");
  const [message, setMessage] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [pending, setPending] = useState(false);

  const editable = listing.status === "draft" || listing.status === "rejected";
  const priceOnly = listing.status === "published" || listing.status === "suspended";
  const imagesEditable = editable || priceOnly;

  function handleError(cause: unknown) {
    if (cause instanceof ApiError && cause.status === 409) {
      setError("Esta ação não é permitida no estado atual do anúncio.");
    } else if (cause instanceof ApiError && cause.status === 400) {
      setError("Confira os dados informados.");
    } else {
      setError("Operação falhou. Tente novamente.");
    }
  }

  async function run(action: () => Promise<void>) {
    setPending(true);
    setError(null);
    setMessage(null);
    try {
      await action();
    } catch (cause) {
      handleError(cause);
    } finally {
      setPending(false);
    }
  }

  async function refresh(api: ReturnType<typeof browserApi>) {
    const updated = await api.getSellerListing(listing.id);
    setListing(updated);
    return updated;
  }

  async function save() {
    await run(async () => {
      const api = browserApi();
      const updated = await api.updateListing(listing.id, {
        ...(editable
          ? {
              title,
              description,
              category_id: categoryId,
              condition,
              free_shipping: freeShipping,
            }
          : {}),
        price_cents: Math.round(Number(price) * 100),
      });
      setListing(updated);
      setMessage("Anúncio salvo.");
      router.refresh();
    });
  }

  async function submit() {
    await run(async () => {
      const updated = await browserApi().submitListing(listing.id);
      setListing(updated);
      setMessage("Anúncio enviado para revisão.");
      router.refresh();
    });
  }

  async function archive() {
    await run(async () => {
      const updated = await browserApi().archiveListing(listing.id);
      setListing(updated);
      setMessage("Anúncio arquivado.");
      router.refresh();
    });
  }

  async function upload(event: React.ChangeEvent<HTMLInputElement>) {
    const file = event.target.files?.[0];
    event.target.value = "";
    if (!file) {
      return;
    }
    await run(async () => {
      const api = browserApi();
      await api.uploadListingImage(listing.id, file);
      await refresh(api);
      setMessage("Imagem adicionada.");
    });
  }

  async function removeImage(imageId: string) {
    await run(async () => {
      const api = browserApi();
      await api.deleteListingImage(listing.id, imageId);
      await refresh(api);
      setMessage("Imagem removida.");
    });
  }

  async function moveImage(index: number, direction: -1 | 1) {
    const target = index + direction;
    if (target < 0 || target >= listing.images.length) {
      return;
    }
    const ids = listing.images.map((image) => image.id);
    [ids[index], ids[target]] = [ids[target], ids[index]];
    await run(async () => {
      const api = browserApi();
      const images: ListingImage[] = await api.reorderListingImages(listing.id, ids);
      setListing((current) => ({ ...current, images }));
      setMessage("Ordem atualizada.");
    });
  }

  async function adjustStock() {
    await run(async () => {
      const api = browserApi();
      const adjustment = await api.adjustInventory(listing.id, { delta: Number(delta), reason });
      setAdjustments((current) => [adjustment, ...current]);
      setListing((current) => ({ ...current, stock: adjustment.quantity }));
      setReason("");
      setMessage("Estoque atualizado.");
    });
  }

  return (
    <div className="seller-editor" data-testid="listing-editor">
      <header className="seller-header">
        <div>
          <p className="eyebrow">Anúncio</p>
          <h1>{listing.title}</h1>
          <p className="seller-listing-meta">
            <span className={`status-pill status-${listing.status}`} data-testid="editor-status">
              {statusLabels[listing.status] ?? listing.status}
            </span>
            <span data-testid="editor-stock">estoque {listing.stock}</span>
          </p>
        </div>
        <a href="/vendedor">Voltar</a>
      </header>

      {listing.status === "rejected" && listing.rejection_reason ? (
        <p className="alert alert-danger" role="alert" data-testid="rejection-reason">
          Rejeitado: {listing.rejection_reason}
        </p>
      ) : null}
      {priceOnly ? (
        <p className="alert" data-testid="review-hint">
          Este anúncio está {statusLabels[listing.status].toLowerCase()}. Só preço e estoque podem mudar; alterações de
          conteúdo exigem novo envio para revisão.
        </p>
      ) : null}
      {message ? <p className="alert alert-ok" data-testid="editor-message">{message}</p> : null}
      {error ? <p className="alert alert-danger" role="alert" data-testid="editor-error">{error}</p> : null}

      <section className="editor-section">
        <h2>Dados do anúncio</h2>
        <div className="listing-form">
          <div className="filter-field">
            <label htmlFor="edit-title">Título</label>
            <input id="edit-title" value={title} disabled={!editable} onChange={(event) => setTitle(event.target.value)} />
          </div>
          <div className="filter-field">
            <label htmlFor="edit-description">Descrição</label>
            <textarea id="edit-description" rows={4} value={description} disabled={!editable} onChange={(event) => setDescription(event.target.value)} />
          </div>
          <div className="filter-field">
            <label htmlFor="edit-category">Categoria</label>
            <select id="edit-category" value={categoryId} disabled={!editable} onChange={(event) => setCategoryId(event.target.value)}>
              {categories.map((category) => (
                <option key={category.id} value={category.id}>
                  {category.name}
                </option>
              ))}
            </select>
          </div>
          <div className="filter-field">
            <label htmlFor="edit-price">Preço (R$)</label>
            <input id="edit-price" type="number" min="0.01" step="0.01" value={price} onChange={(event) => setPrice(event.target.value)} />
          </div>
          <div className="filter-field">
            <label htmlFor="edit-condition">Condição</label>
            <select id="edit-condition" value={condition} disabled={!editable} onChange={(event) => setCondition(event.target.value as "new" | "used")}>
              <option value="used">Usado</option>
              <option value="new">Novo</option>
            </select>
          </div>
          <label className="filter-check">
            <input type="checkbox" checked={freeShipping} disabled={!editable} onChange={(event) => setFreeShipping(event.target.checked)} />
            Frete grátis
          </label>
          <div className="editor-actions">
            <button type="button" className="primary-action" disabled={pending} onClick={save} data-testid="save-listing">
              Salvar
            </button>
            {editable ? (
              <button type="button" className="secondary-action" disabled={pending} onClick={submit} data-testid="submit-listing">
                {listing.status === "rejected" ? "Reenviar para revisão" : "Enviar para revisão"}
              </button>
            ) : null}
            {listing.status === "draft" || listing.status === "published" || listing.status === "suspended" ? (
              <button type="button" className="secondary-action" disabled={pending} onClick={archive} data-testid="archive-listing">
                Arquivar
              </button>
            ) : null}
          </div>
        </div>
      </section>

      <section className="editor-section">
        <h2>Imagens</h2>
        <p className="form-hint">A primeira imagem é a capa exibida na vitrine. Máximo de 8 imagens, 5 MiB cada.</p>
        {imagesEditable ? (
          <input type="file" accept="image/jpeg,image/png,image/webp" onChange={upload} disabled={pending} data-testid="image-input" />
        ) : (
          <p className="form-hint">Imagens não podem ser alteradas em revisão.</p>
        )}
        <ul className="image-list" data-testid="image-list">
          {listing.images.map((image, index) => (
            <li key={image.id} className="image-item" data-testid="image-item">
              <img src={image.url} alt="" />
              <div className="image-item-actions">
                <span>{index === 0 ? "Capa" : `Posição ${index + 1}`}</span>
                <button type="button" disabled={pending || index === 0} onClick={() => moveImage(index, -1)} data-testid="image-up">
                  ↑
                </button>
                <button
                  type="button"
                  disabled={pending || index === listing.images.length - 1}
                  onClick={() => moveImage(index, 1)}
                  data-testid="image-down"
                >
                  ↓
                </button>
                <button type="button" disabled={pending} onClick={() => removeImage(image.id)} data-testid="image-remove">
                  Remover
                </button>
              </div>
            </li>
          ))}
        </ul>
      </section>

      <section className="editor-section">
        <h2>Estoque</h2>
        <div className="stock-form">
          <div className="filter-field">
            <label htmlFor="stock-delta">Ajuste</label>
            <input id="stock-delta" type="number" value={delta} onChange={(event) => setDelta(event.target.value)} data-testid="stock-delta" />
          </div>
          <div className="filter-field">
            <label htmlFor="stock-reason">Motivo</label>
            <input id="stock-reason" minLength={3} value={reason} onChange={(event) => setReason(event.target.value)} data-testid="stock-reason" />
          </div>
          <button type="button" className="primary-action" disabled={pending || reason.trim().length < 3} onClick={adjustStock} data-testid="adjust-stock">
            Ajustar estoque
          </button>
        </div>
        <ul className="stock-history" data-testid="stock-history">
          {adjustments.map((adjustment) => (
            <li key={adjustment.id}>
              <span>
                {adjustment.delta > 0 ? "+" : ""}
                {adjustment.delta} → {adjustment.quantity}
              </span>
              <span>{adjustment.reason}</span>
            </li>
          ))}
        </ul>
      </section>
    </div>
  );
}
