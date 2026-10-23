import { EmptyState } from "@mercado/ui";
import { requireSeller } from "@/lib/session";

export const dynamic = "force-dynamic";

const statusLabels: Record<string, string> = {
  draft: "Rascunho",
  pending_review: "Em revisão",
  published: "Publicado",
  rejected: "Rejeitado",
  suspended: "Suspenso",
  archived: "Arquivado",
};

export default async function SellerDashboard() {
  const { api, user } = await requireSeller();
  const listings = await api.listSellerListings();

  return (
    <div className="seller-page" data-testid="seller-dashboard">
      <header className="seller-header">
        <div>
          <p className="eyebrow">Área do vendedor</p>
          <h1>Olá, {user.name}</h1>
          <p className="lede">Gerencie seus anúncios, estoque e imagens.</p>
        </div>
        <a className="primary-action" href="/vendedor/novo">
          Novo anúncio
        </a>
      </header>

      {listings.length === 0 ? (
        <EmptyState title="Nenhum anúncio ainda" message="Crie seu primeiro anúncio para começar a vender." />
      ) : (
        <ul className="seller-listings" data-testid="seller-listings">
          {listings.map((listing) => (
            <li key={listing.id} className="seller-listing">
              <div>
                <a className="seller-listing-title" href={`/vendedor/${listing.id}`} data-testid="seller-listing-link">
                  {listing.title}
                </a>
                <p className="seller-listing-meta">
                  {listing.category.name} · R$ {(listing.price_cents / 100).toFixed(2)} · estoque {listing.stock}
                </p>
              </div>
              <span className={`status-pill status-${listing.status}`} data-testid="seller-listing-status">
                {statusLabels[listing.status] ?? listing.status}
              </span>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
