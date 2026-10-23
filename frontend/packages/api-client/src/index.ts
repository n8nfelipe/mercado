import type { components } from "./schema.js";

export type Status = components["schemas"]["Status"];
export type Health = components["schemas"]["Health"];
export type Readiness = components["schemas"]["Readiness"];
export type ErrorResponse = components["schemas"]["ErrorResponse"];
export type ErrorDetail = components["schemas"]["ErrorDetail"];
export type FieldError = components["schemas"]["FieldError"];
export type User = components["schemas"]["User"];
export type Role = components["schemas"]["Role"];
export type RegisterRequest = components["schemas"]["RegisterRequest"];
export type LoginRequest = components["schemas"]["LoginRequest"];
export type SellerOnboardingRequest = components["schemas"]["SellerOnboardingRequest"];
export type SellerProfile = components["schemas"]["SellerProfile"];
export type PublicSeller = components["schemas"]["PublicSeller"];
export type Category = components["schemas"]["Category"];
export type CategoryInput = components["schemas"]["CategoryInput"];
export type Condition = components["schemas"]["Condition"];
export type Sort = components["schemas"]["Sort"];
export type ListingStatus = components["schemas"]["ListingStatus"];
export type ListingInput = components["schemas"]["ListingInput"];
export type ListingPatch = components["schemas"]["ListingPatch"];
export type ListingImage = components["schemas"]["ListingImage"];
export type ImageOrderRequest = components["schemas"]["ImageOrderRequest"];
export type InventoryAdjustment = components["schemas"]["InventoryAdjustment"];
export type InventoryAdjustmentRequest = components["schemas"]["InventoryAdjustmentRequest"];
export type ListingSummary = components["schemas"]["ListingSummary"];
export type ListingDetail = components["schemas"]["ListingDetail"];
export type ListingPage = components["schemas"]["ListingPage"];
export type SellerListing = components["schemas"]["SellerListing"];
export type ModerationRequest = components["schemas"]["ModerationRequest"];

export interface ApiClientOptions {
  baseUrl: string;
  apiKey?: string;
  headers?: Record<string, string>;
  fetch?: typeof fetch;
}

export interface RequestOptions {
  signal?: AbortSignal;
  headers?: Record<string, string>;
}

export interface SearchListingsParams {
  q?: string;
  category_id?: string;
  min_price_cents?: number;
  max_price_cents?: number;
  condition?: Condition;
  free_shipping?: boolean;
  sort?: Sort;
  page?: number;
  page_size?: number;
}

export class ApiError extends Error {
  readonly status: number;
  readonly code: string;
  readonly correlationId?: string;
  readonly details?: FieldError[];

  constructor(status: number, body?: ErrorResponse, fallbackMessage = "") {
    const detail = body?.error;
    super(detail?.message ?? (fallbackMessage || `HTTP ${status}`));
    this.name = "ApiError";
    this.status = status;
    this.code = detail?.code ?? "http_error";
    this.correlationId = body?.correlation_id;
    this.details = detail?.details;
  }
}

export function createClient(options: ApiClientOptions) {
  const baseUrl = options.baseUrl.replace(/\/+$/, "");
  const doFetch = options.fetch ?? globalThis.fetch;

  async function request<T>(
    method: string,
    path: string,
    requestOptions: RequestOptions & { json?: unknown; formData?: FormData } = {},
  ): Promise<T> {
    const headers = new Headers(options.headers);
    headers.set("Accept", "application/json");
    if (options.apiKey) {
      headers.set("X-API-Key", options.apiKey);
    }
    for (const [name, value] of Object.entries(requestOptions.headers ?? {})) {
      headers.set(name, value);
    }

    let body: BodyInit | undefined;
    if (requestOptions.formData) {
      body = requestOptions.formData;
    } else if (requestOptions.json !== undefined) {
      headers.set("Content-Type", "application/json");
      body = JSON.stringify(requestOptions.json);
    }

    const response = await doFetch(`${baseUrl}${path}`, {
      method,
      headers,
      body,
      signal: requestOptions.signal,
      credentials: "include",
    });

    if (!response.ok) {
      let errorBody: ErrorResponse | undefined;
      try {
        errorBody = (await response.json()) as ErrorResponse;
      } catch {
        errorBody = undefined;
      }
      throw new ApiError(response.status, errorBody, response.statusText);
    }
    if (response.status === 204 || response.headers.get("content-length") === "0") {
      return undefined as T;
    }
    return (await response.json()) as T;
  }

  function getJSON<T>(path: string, requestOptions?: RequestOptions): Promise<T> {
    return request<T>("GET", path, requestOptions);
  }

  function queryString(params: object): string {
    const search = new URLSearchParams();
    for (const [key, value] of Object.entries(params)) {
      if (value !== undefined && value !== "") {
        search.set(key, String(value));
      }
    }
    const query = search.toString();
    return query ? `?${query}` : "";
  }

  return {
    getStatus: (requestOptions?: RequestOptions) => getJSON<Status>("/v1/status", requestOptions),
    getHealth: (requestOptions?: RequestOptions) => getJSON<Health>("/healthz", requestOptions),
    getReadiness: (requestOptions?: RequestOptions) => getJSON<Readiness>("/readyz", requestOptions),

    register: (input: RegisterRequest, requestOptions?: RequestOptions) =>
      request<User>("POST", "/v1/auth/register", { ...requestOptions, json: input }),
    login: (input: LoginRequest, requestOptions?: RequestOptions) =>
      request<User>("POST", "/v1/auth/login", { ...requestOptions, json: input }),
    logout: (requestOptions?: RequestOptions) =>
      request<void>("POST", "/v1/auth/logout", requestOptions),
    getMe: (requestOptions?: RequestOptions) => getJSON<User>("/v1/auth/me", requestOptions),

    becomeSeller: (input: SellerOnboardingRequest, requestOptions?: RequestOptions) =>
      request<SellerProfile>("POST", "/v1/seller/onboarding", { ...requestOptions, json: input }),
    getPublicSeller: (sellerId: string, requestOptions?: RequestOptions) =>
      getJSON<PublicSeller>(`/v1/sellers/${encodeURIComponent(sellerId)}`, requestOptions),

    listPublicCategories: (requestOptions?: RequestOptions) =>
      getJSON<Category[]>("/v1/categories", requestOptions),
    createCategory: (input: CategoryInput, requestOptions?: RequestOptions) =>
      request<Category>("POST", "/v1/admin/categories", { ...requestOptions, json: input }),
    updateCategory: (categoryId: string, input: CategoryInput, requestOptions?: RequestOptions) =>
      request<Category>("PATCH", `/v1/admin/categories/${encodeURIComponent(categoryId)}`, {
        ...requestOptions,
        json: input,
      }),
    deleteCategory: (categoryId: string, requestOptions?: RequestOptions) =>
      request<void>("DELETE", `/v1/admin/categories/${encodeURIComponent(categoryId)}`, requestOptions),

    searchListings: (params: SearchListingsParams = {}, requestOptions?: RequestOptions) =>
      getJSON<ListingPage>(`/v1/catalog/listings${queryString(params)}`, requestOptions),
    getPublicListing: (listingId: string, requestOptions?: RequestOptions) =>
      getJSON<ListingDetail>(`/v1/listings/${encodeURIComponent(listingId)}`, requestOptions),

    listSellerListings: (requestOptions?: RequestOptions) =>
      getJSON<SellerListing[]>("/v1/seller/listings", requestOptions),
    createListing: (input: ListingInput, requestOptions?: RequestOptions) =>
      request<SellerListing>("POST", "/v1/seller/listings", { ...requestOptions, json: input }),
    getSellerListing: (listingId: string, requestOptions?: RequestOptions) =>
      getJSON<SellerListing>(`/v1/seller/listings/${encodeURIComponent(listingId)}`, requestOptions),
    updateListing: (listingId: string, input: ListingPatch, requestOptions?: RequestOptions) =>
      request<SellerListing>("PATCH", `/v1/seller/listings/${encodeURIComponent(listingId)}`, {
        ...requestOptions,
        json: input,
      }),
    submitListing: (listingId: string, requestOptions?: RequestOptions) =>
      request<SellerListing>(
        "POST",
        `/v1/seller/listings/${encodeURIComponent(listingId)}/submit`,
        requestOptions,
      ),
    archiveListing: (listingId: string, requestOptions?: RequestOptions) =>
      request<SellerListing>(
        "POST",
        `/v1/seller/listings/${encodeURIComponent(listingId)}/archive`,
        requestOptions,
      ),
    uploadListingImage: (listingId: string, file: File, requestOptions?: RequestOptions) => {
      const formData = new FormData();
      formData.set("file", file);
      return request<ListingImage>(
        "POST",
        `/v1/seller/listings/${encodeURIComponent(listingId)}/images`,
        { ...requestOptions, formData },
      );
    },
    reorderListingImages: (
      listingId: string,
      imageIds: string[],
      requestOptions?: RequestOptions,
    ) =>
      request<ListingImage[]>(
        "PATCH",
        `/v1/seller/listings/${encodeURIComponent(listingId)}/images/order`,
        { ...requestOptions, json: { image_ids: imageIds } satisfies ImageOrderRequest },
      ),
    deleteListingImage: (listingId: string, imageId: string, requestOptions?: RequestOptions) =>
      request<void>(
        "DELETE",
        `/v1/seller/listings/${encodeURIComponent(listingId)}/images/${encodeURIComponent(imageId)}`,
        requestOptions,
      ),
    listInventoryAdjustments: (listingId: string, requestOptions?: RequestOptions) =>
      getJSON<InventoryAdjustment[]>(
        `/v1/seller/listings/${encodeURIComponent(listingId)}/inventory`,
        requestOptions,
      ),
    adjustInventory: (
      listingId: string,
      input: InventoryAdjustmentRequest,
      requestOptions?: RequestOptions,
    ) =>
      request<InventoryAdjustment>(
        "POST",
        `/v1/seller/listings/${encodeURIComponent(listingId)}/inventory`,
        { ...requestOptions, json: input },
      ),

    listPendingListings: (requestOptions?: RequestOptions) =>
      getJSON<SellerListing[]>("/v1/admin/listings/pending", requestOptions),
    publishListing: (listingId: string, requestOptions?: RequestOptions) =>
      request<SellerListing>(
        "POST",
        `/v1/admin/listings/${encodeURIComponent(listingId)}/publish`,
        requestOptions,
      ),
    rejectListing: (listingId: string, input: ModerationRequest, requestOptions?: RequestOptions) =>
      request<SellerListing>(
        "POST",
        `/v1/admin/listings/${encodeURIComponent(listingId)}/reject`,
        { ...requestOptions, json: input },
      ),
    suspendListing: (listingId: string, input: ModerationRequest, requestOptions?: RequestOptions) =>
      request<SellerListing>(
        "POST",
        `/v1/admin/listings/${encodeURIComponent(listingId)}/suspend`,
        { ...requestOptions, json: input },
      ),
  };
}

export type ApiClient = ReturnType<typeof createClient>;
