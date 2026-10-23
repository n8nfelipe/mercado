import type { ReactNode } from "react";

export interface SiteHeaderProps {
  search?: ReactNode;
  right?: ReactNode;
}

export function SiteHeader({ search, right }: SiteHeaderProps) {
  return (
    <header className="site-header">
      <a className="site-brand" href="/">
        Mercado
      </a>
      <nav className="site-nav" aria-label="Principal">
        <a href="/">Vitrine</a>
        <a href="/vendedor">Anunciar</a>
      </nav>
      {search}
      <div className="site-session">{right}</div>
    </header>
  );
}
