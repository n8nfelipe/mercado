export interface PaginationProps {
  page: number;
  totalPages: number;
  makeHref: (page: number) => string;
}

export function Pagination({ page, totalPages, makeHref }: PaginationProps) {
  if (totalPages <= 1) {
    return null;
  }
  const pages = new Set<number>([1, totalPages, page - 1, page, page + 1]);
  const visible = [...pages].filter((value) => value >= 1 && value <= totalPages).sort((a, b) => a - b);

  return (
    <nav className="pagination" aria-label="Paginação" data-testid="pagination">
      {page > 1 ? (
        <a href={makeHref(page - 1)} rel="prev">
          Anterior
        </a>
      ) : (
        <span className="pagination-disabled">Anterior</span>
      )}
      {visible.map((value) => (
        <a
          key={value}
          href={makeHref(value)}
          aria-current={value === page ? "page" : undefined}
          className={value === page ? "pagination-current" : undefined}
        >
          {value}
        </a>
      ))}
      {page < totalPages ? (
        <a href={makeHref(page + 1)} rel="next">
          Próxima
        </a>
      ) : (
        <span className="pagination-disabled">Próxima</span>
      )}
    </nav>
  );
}
