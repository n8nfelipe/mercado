export interface UnavailablePanelProps {
  message: string;
}

export function UnavailablePanel({ message }: UnavailablePanelProps) {
  return (
    <section className="status-card status-card-error" role="alert">
      <div className="status-heading">
        <span className="status-dot" aria-hidden="true" />
        <h2>API indisponível</h2>
      </div>
      <p data-testid="status-error">{message}</p>
      <p className="hint">Verifique se o backend está em execução e recarregue a página.</p>
    </section>
  );
}
