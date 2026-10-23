import { notFound } from "next/navigation";
import { ApiError, type ListingDetail } from "@mercado/api-client";
import { conditionLabel, formatPrice } from "@mercado/ui";
import { serverApi } from "@/lib/api-server";

export const dynamic = "force-dynamic";

export default async function ListingPage({ params }: { params: Promise<{ listingId: string }> }) {
  const { listingId } = await params;
  let listing: ListingDetail;
  try {
    listing = await serverApi().getPublicListing(listingId);
  } catch (cause) {
    if (cause instanceof ApiError && cause.status === 404) {
      notFound();
    }
    throw cause;
  }

  const available = listing.stock > 0;

  return (
    <article className="detail" data-testid="listing-detail">
      <nav className="breadcrumb" aria-label="Navegação">
        <a href="/">Vitrine</a>
        <span aria-hidden="true">/</span>
        <a href={`/?category_id=${listing.category.id}`}>{listing.category.name}</a>
      </nav>

      <div className="detail-layout">
        <div className="detail-gallery">
          {listing.images.length > 0 ? (
            <>
              <img className="detail-cover" src={listing.images[0].url} alt={listing.title} />
              {listing.images.length > 1 ? (
                <ul className="detail-thumbs">
                  {listing.images.slice(1).map((image) => (
                    <li key={image.id}>
                      <img src={image.url} alt="" loading="lazy" />
                    </li>
                  ))}
                </ul>
              ) : null}
            </>
          ) : (
            <div className="detail-cover detail-cover-empty">sem foto</div>
          )}
        </div>

        <div className="detail-info">
          <p className="eyebrow">{listing.category.name}</p>
          <h1 data-testid="detail-title">{listing.title}</h1>
          <p className="detail-price" data-testid="detail-price">
            {formatPrice(listing.price_cents, listing.currency)}
          </p>
          <dl className="detail-facts">
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
              <dd data-testid="detail-stock">{available ? listing.stock : "Indisponível"}</dd>
            </div>
          </dl>
          <button type="button" className="primary-action" disabled={!available}>
            {available ? "Comprar" : "Indisponível"}
          </button>
          <section className="detail-description">
            <h2>Descrição</h2>
            <p>{listing.description}</p>
          </section>
          <section className="detail-seller">
            <h2>Vendedor</h2>
            <p data-testid="detail-seller">{listing.seller.display_name}</p>
          </section>
        </div>
      </div>
    </article>
  );
}
