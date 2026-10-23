import type { Status } from "@mercado/api-client";

export interface StatusCardProps {
  status: Status;
}

export function StatusCard({ status }: StatusCardProps) {
  return (
    <section className="status-card" aria-labelledby="status-title">
      <div className="status-heading">
        <span className="status-dot" aria-hidden="true" />
        <h2 id="status-title">API online</h2>
      </div>
      <dl className="status-list">
        <div>
          <dt>Serviço</dt>
          <dd data-testid="status-service">{status.service}</dd>
        </div>
        <div>
          <dt>Versão</dt>
          <dd data-testid="status-version">{status.version}</dd>
        </div>
        <div>
          <dt>Commit</dt>
          <dd data-testid="status-commit">{status.commit}</dd>
        </div>
      </dl>
    </section>
  );
}
