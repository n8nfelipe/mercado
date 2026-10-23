import type { ListingSummary } from "@mercado/api-client";
import { conditionLabel, formatPrice } from "./format.js";

export interface ListingCardProps {
  listing: ListingSummary;
  href: string;
}

export function ListingCard({ listing, href }: ListingCardProps) {
  return (
    <a className="listing-card" href={href} data-testid="listing-card">
      <div className="listing-card-media">
        {listing.cover_image_url ? (
          <img src={listing.cover_image_url} alt="" loading="lazy" />
        ) : (
          <span className="listing-card-placeholder" aria-hidden="true">
            sem foto
          </span>
        )}
      </div>
      <div className="listing-card-body">
        <h3 className="listing-card-title">{listing.title}</h3>
        <p className="listing-card-price" data-testid="listing-price">
          {formatPrice(listing.price_cents, listing.currency)}
        </p>
        <dl className="listing-card-meta">
          <div>
            <dt>Condição</dt>
            <dd>{conditionLabel(listing.condition)}</dd>
          </div>
          <div>
            <dt>Frete</dt>
            <dd>{listing.free_shipping ? "Grátis" : "A calcular"}</dd>
          </div>
          <div>
            <dt>Estoque</dt>
            <dd data-testid="listing-stock">{listing.stock > 0 ? listing.stock : "Indisponível"}</dd>
          </div>
        </dl>
        <p className="listing-card-seller">{listing.seller.display_name}</p>
      </div>
    </a>
  );
}
