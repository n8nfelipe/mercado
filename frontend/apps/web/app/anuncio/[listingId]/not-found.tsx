import { EmptyState } from "@mercado/ui";

export default function ListingNotFound() {
  return (
    <EmptyState
      title="Anúncio não encontrado"
      message="Este anúncio pode ter sido removido, suspenso ou nunca existiu."
    />
  );
}
