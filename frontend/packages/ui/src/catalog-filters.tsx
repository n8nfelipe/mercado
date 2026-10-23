import type { Category } from "@mercado/api-client";

export interface CatalogFilterValues {
  q?: string;
  category_id?: string;
  condition?: string;
  free_shipping?: string;
  min_price?: string;
  max_price?: string;
  sort?: string;
}

export interface CatalogFiltersProps {
  categories: Category[];
  values: CatalogFilterValues;
}

export function CatalogFilters({ categories, values }: CatalogFiltersProps) {
  return (
    <form className="catalog-filters" method="get" action="/" role="search">
      <div className="filter-field filter-field-wide">
        <label htmlFor="q">Busca</label>
        <input id="q" name="q" type="search" defaultValue={values.q ?? ""} placeholder="O que você procura?" />
      </div>
      <div className="filter-field">
        <label htmlFor="category_id">Categoria</label>
        <select id="category_id" name="category_id" defaultValue={values.category_id ?? ""}>
          <option value="">Todas</option>
          {categories.map((category) => (
            <option key={category.id} value={category.id}>
              {category.name}
            </option>
          ))}
        </select>
      </div>
      <div className="filter-field">
        <label htmlFor="condition">Condição</label>
        <select id="condition" name="condition" defaultValue={values.condition ?? ""}>
          <option value="">Qualquer</option>
          <option value="new">Novo</option>
          <option value="used">Usado</option>
        </select>
      </div>
      <div className="filter-field">
        <label htmlFor="sort">Ordenar</label>
        <select id="sort" name="sort" defaultValue={values.sort ?? "relevance"}>
          <option value="relevance">Relevância</option>
          <option value="newest">Mais recentes</option>
          <option value="price_asc">Menor preço</option>
          <option value="price_desc">Maior preço</option>
        </select>
      </div>
      <div className="filter-field">
        <label htmlFor="min_price">Preço mín. (R$)</label>
        <input id="min_price" name="min_price" type="number" min="0" step="1" defaultValue={values.min_price ?? ""} />
      </div>
      <div className="filter-field">
        <label htmlFor="max_price">Preço máx. (R$)</label>
        <input id="max_price" name="max_price" type="number" min="0" step="1" defaultValue={values.max_price ?? ""} />
      </div>
      <label className="filter-check">
        <input type="checkbox" name="free_shipping" value="true" defaultChecked={values.free_shipping === "true"} />
        Frete grátis
      </label>
      <div className="filter-actions">
        <button type="submit">Filtrar</button>
        <a href="/">Limpar</a>
      </div>
    </form>
  );
}
