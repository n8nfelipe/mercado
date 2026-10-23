import { ApiError } from "@mercado/api-client";
import { notFound } from "next/navigation";
import { requireSeller } from "@/lib/session";
import { ListingEditor } from "./listing-editor";

export const dynamic = "force-dynamic";

export default async function SellerListingPage({ params }: { params: Promise<{ listingId: string }> }) {
  const { listingId } = await params;
  const { api } = await requireSeller();

  try {
    const [listing, categories, adjustments] = await Promise.all([
      api.getSellerListing(listingId),
      api.listPublicCategories(),
      api.listInventoryAdjustments(listingId),
    ]);
    return <ListingEditor listing={listing} categories={categories} adjustments={adjustments} />;
  } catch (cause) {
    if (cause instanceof ApiError && cause.status === 404) {
      notFound();
    }
    throw cause;
  }
}
