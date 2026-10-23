import { ApiError, type Category, type Condition, type ListingPage, type Sort } from "@mercado/api-client";
import { CatalogFilters, EmptyState, ListingCard, Pagination, UnavailablePanel } from "@mercado/ui";
import { serverApi } from "@/lib/api-server";

export const dynamic = "force-dynamic";

type SearchParams = Record<string, string | string[] | undefined>;

const sorted = new Set<Sort>(["relevance", "price_asc", "price_desc", "newest"]);
const conditions = new Set<Condition>(["new", "used"]);

function first(value: string | string[] | undefined): string {
  if (Array.isArray(value)) {
    return value[0] ?? "";
  }
  return value ?? "";
}

function priceInCents(value: string): number | undefined {
  if (!value.trim()) {
    return undefined;
  }
  const parsed = Number(value);
  if (!Number.isFinite(parsed) || parsed < 0) {
    return undefined;
  }
  return Math.round(parsed * 100);
}

export default async function Home({ searchParams }: { searchParams: Promise<SearchParams> }) {
  const params = await searchParams;
  const q = first(params.q).trim();
  const categoryId = first(params.category_id);
  const condition = first(params.condition);
  const freeShipping = first(params.free_shipping) === "true";
  const minPrice = first(params.min_price);
  const maxPrice = first(params.max_price);
  const sortParam = first(params.sort);
  const page = Math.max(1, Number.parseInt(first(params.page) || "1", 10) || 1);
  const pageSize = 12;

  const api = serverApi();
  let categories: Category[] = [];
  let results: ListingPage | undefined;
  let error: string | undefined;

  try {
    [categories, results] = await Promise.all([
      api.listPublicCategories(),
      api.searchListings({
        q: q || undefined,
        category_id: categoryId || undefined,
        condition: conditions.has(condition as Condition) ? (condition as Condition) : undefined,
        free_shipping: freeShipping ? true : undefined,
        min_price_cents: priceInCents(minPrice),
        max_price_cents: priceInCents(maxPrice),
        sort: sorted.has(sortParam as Sort) ? (sortParam as Sort) : undefined,
        page,
        page_size: pageSize,
      }),
    ]);
  } catch (cause) {
    if (cause instanceof ApiError) {
      error = `A vitrine respondeu com ${cause.status} (${cause.code}).`;
    } else {
      error = "Não foi possível carregar a vitrine.";
    }
  }

  function makeHref(targetPage: number): string {
    const query = new URLSearchParams();
    for (const [key, value] of Object.entries({ q, category_id: categoryId, condition, min_price: minPrice, max_price: maxPrice, sort: sortParam })) {
      if (value) {
        query.set(key, value);
      }
    }
    if (freeShipping) {
      query.set("free_shipping", "true");
    }
    query.set("page", String(targetPage));
    return `/?${query.toString()}`;
  }

  return (
    <div className="page">
      <section className="catalog-hero">
        <p className="eyebrow">Vitrine</p>
        <h1 className="catalog-title">Encontre o próximo achado</h1>
        <p className="lede">
          Anúncios publicados por vendedores locais, com estoque real e frete transparente.
        </p>
      </section>

      <div className="catalog-layout">
        <aside className="catalog-sidebar">
          <CatalogFilters
            categories={categories}
            values={{
              q,
              category_id: categoryId,
              condition,
              free_shipping: freeShipping ? "true" : "",
              min_price: minPrice,
              max_price: maxPrice,
              sort: sortParam || "relevance",
            }}
          />
        </aside>

        <section className="catalog-results" aria-live="polite">
          {error ? (
            <UnavailablePanel message={error} />
          ) : results && results.items.length > 0 ? (
            <>
              <div className="results-header">
                <p data-testid="results-total">
                  {results.total} {results.total === 1 ? "anúncio" : "anúncios"}
                </p>
                <p>
                  Página {results.page} de {results.total_pages || 1}
                </p>
              </div>
              <div className="listing-grid" data-testid="listing-grid">
                {results.items.map((listing) => (
                  <ListingCard key={listing.id} listing={listing} href={`/anuncio/${listing.id}`} />
                ))}
              </div>
              <Pagination page={results.page} totalPages={results.total_pages} makeHref={makeHref} />
            </>
          ) : (
            <EmptyState
              title="Nenhum anúncio encontrado"
              message="Ajuste os filtros ou tente outra busca para ver mais resultados."
            />
          )}
        </section>
      </div>
    </div>
  );
}
