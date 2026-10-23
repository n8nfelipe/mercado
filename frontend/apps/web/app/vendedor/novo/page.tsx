import { serverApi } from "@/lib/api-server";
import { requireSeller } from "@/lib/session";
import { NewListingForm } from "./new-listing-form";

export const dynamic = "force-dynamic";

export default async function NewListingPage() {
  await requireSeller();
  const categories = await serverApi().listPublicCategories();

  return (
    <div className="seller-page">
      <header className="seller-header">
        <div>
          <p className="eyebrow">Novo anúncio</p>
          <h1>Criar anúncio</h1>
          <p className="lede">Preencha os dados e envie para revisão quando estiver pronto.</p>
        </div>
      </header>
      <NewListingForm categories={categories} />
    </div>
  );
}
