import crypto from "node:crypto";
import http from "node:http";

let failing = false;

const bikeCategory = { id: "01900000-0000-7000-8000-0000000000c1", name: "Bicicletas", slug: "bicicletas", parent_id: null, is_active: true, created_at: "2026-01-01T00:00:00Z" };
const accessoryCategory = { id: "01900000-0000-7000-8000-0000000000c2", name: "Acessórios", slug: "acessorios", parent_id: null, is_active: true, created_at: "2026-01-01T00:00:00Z" };
const categories = [bikeCategory, accessoryCategory];

const seedSeller = { id: "01900000-0000-7000-8000-000000000091", display_name: "Loja do Bairro" };
const suspendedListingId = "01900000-0000-7000-8000-0000000000a9";

function seedListing(id, title, priceCents, condition, freeShipping, stock, category, publishedAt) {
  return {
    id,
    title,
    description: "Descrição detalhada do anúncio.",
    price_cents: priceCents,
    currency: "BRL",
    condition,
    free_shipping: freeShipping,
    stock,
    status: "published",
    rejection_reason: null,
    category,
    seller: seedSeller,
    images: [],
    cover_image_url: null,
    submitted_at: null,
    published_at: publishedAt,
    created_at: publishedAt,
    updated_at: publishedAt,
    created: false,
  };
}

const catalog = [
  seedListing("01900000-0000-7000-8000-0000000000a1", "Bicicleta Aro 29 Usada", 120000, "used", true, 2, bikeCategory, "2026-01-02T00:00:00Z"),
  seedListing("01900000-0000-7000-8000-0000000000a2", "Capacete Novo", 8000, "new", false, 0, accessoryCategory, "2026-01-03T00:00:00Z"),
  seedListing("01900000-0000-7000-8000-0000000000a3", "Bicicleta Infantil Aro 20", 35000, "used", true, 5, bikeCategory, "2026-01-04T00:00:00Z"),
  { ...seedListing(suspendedListingId, "Anúncio Suspenso", 1000, "used", false, 1, bikeCategory, "2026-01-05T00:00:00Z"), status: "suspended" },
];

const sessions = new Map();
const inventory = new Map();
const ONE_PIXEL_PNG = Buffer.from(
  "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=",
  "base64",
);

function json(response, status, body, headers = {}) {
  response.writeHead(status, { "content-type": "application/json", ...headers });
  response.end(JSON.stringify(body));
}

function error(response, status, code, message) {
  json(response, status, { error: { code, message }, correlation_id: "e2e" });
}

function readBody(request) {
  return new Promise((resolve) => {
    let body = "";
    request.on("data", (chunk) => {
      body += chunk;
    });
    request.on("end", () => {
      try {
        resolve(body ? JSON.parse(body) : {});
      } catch {
        resolve({});
      }
    });
  });
}

async function readRaw(request) {
  return new Promise((resolve) => {
    const chunks = [];
    request.on("data", (chunk) => chunks.push(chunk));
    request.on("end", () => resolve(Buffer.concat(chunks)));
  });
}

function sessionOf(request) {
  const cookies = request.headers.cookie ?? "";
  for (const part of cookies.split(";")) {
    const [name, value] = part.trim().split("=");
    if (name === "mercado_session" && sessions.has(value)) {
      return { token: value, ...sessions.get(value) };
    }
  }
  return null;
}

function currentUser(session) {
  const { token, ...user } = session;
  return user;
}

function summary(listing) {
  const { description, images, status, rejection_reason, submitted_at, created_at, updated_at, created, ...rest } = listing;
  return { ...rest, cover_image_url: images[0]?.url ?? null };
}

function sellerView(listing) {
  const { created, cover_image_url, ...rest } = listing;
  return rest;
}

function categoryById(id) {
  return categories.find((category) => category.id === id) ?? bikeCategory;
}

function sendCatalog(response, url) {
  const query = url.searchParams;
  let items = catalog.filter((listing) => listing.status === "published" && !listing.created);
  const q = (query.get("q") ?? "").toLowerCase();
  if (q) {
    items = items.filter((listing) => listing.title.toLowerCase().includes(q));
  }
  const categoryId = query.get("category_id");
  if (categoryId) {
    items = items.filter((listing) => listing.category.id === categoryId);
  }
  const condition = query.get("condition");
  if (condition) {
    items = items.filter((listing) => listing.condition === condition);
  }
  if (query.get("free_shipping") === "true") {
    items = items.filter((listing) => listing.free_shipping);
  }
  const minPrice = Number(query.get("min_price_cents") ?? "");
  if (Number.isFinite(minPrice) && minPrice > 0) {
    items = items.filter((listing) => listing.price_cents >= minPrice);
  }
  const maxPrice = Number(query.get("max_price_cents") ?? "");
  if (Number.isFinite(maxPrice) && maxPrice > 0) {
    items = items.filter((listing) => listing.price_cents <= maxPrice);
  }
  const sort = query.get("sort");
  if (sort === "price_asc") {
    items = [...items].sort((a, b) => a.price_cents - b.price_cents);
  } else if (sort === "price_desc") {
    items = [...items].sort((a, b) => b.price_cents - a.price_cents);
  } else {
    items = [...items].sort((a, b) => (a.published_at < b.published_at ? 1 : -1));
  }
  const page = Math.max(1, Number(query.get("page") ?? "1"));
  const pageSize = Math.max(1, Math.min(50, Number(query.get("page_size") ?? "20")));
  const total = items.length;
  json(response, 200, {
    items: items.slice((page - 1) * pageSize, (page - 1) * pageSize + pageSize).map(summary),
    page,
    page_size: pageSize,
    total,
    total_pages: total === 0 ? 0 : Math.ceil(total / pageSize),
  });
}

function sellerListing(response, session, id) {
  const listing = catalog.find((item) => item.id === id && item.created && item.seller.id === session.sellerId);
  if (!listing) {
    error(response, 404, "not_found", "listing not found");
    return null;
  }
  return listing;
}

function compaction(listing) {
  listing.images.forEach((image, index) => {
    image.position = index;
  });
}

const server = http.createServer(async (request, response) => {
  const url = new URL(request.url, "http://127.0.0.1:18082");

  if (request.method === "POST" && url.pathname === "/toggle") {
    failing = !failing;
    response.writeHead(204);
    response.end();
    return;
  }

  if (failing) {
    error(response, 503, "unavailable", "simulated outage");
    return;
  }

  if (request.method === "GET" && url.pathname.startsWith("/v1/media/")) {
    response.writeHead(200, { "content-type": "image/png", "cache-control": "public, max-age=60" });
    response.end(ONE_PIXEL_PNG);
    return;
  }

  if (request.method === "POST" && url.pathname === "/v1/auth/register") {
    const body = await readBody(request);
    const token = crypto.randomUUID();
    sessions.set(token, {
      id: crypto.randomUUID(),
      name: body.name,
      email: body.email,
      roles: ["buyer"],
      created_at: "2026-01-01T00:00:00Z",
      sellerId: null,
      sellerName: null,
    });
    json(response, 201, currentUser({ token, ...sessions.get(token) }), {
      "set-cookie": `mercado_session=${token}; Path=/; HttpOnly; SameSite=Lax`,
    });
    return;
  }

  if (request.method === "POST" && url.pathname === "/v1/auth/login") {
    const body = await readBody(request);
    if (!body.email || !body.password || body.email === "erro@example.com") {
      error(response, 401, "invalid_credentials", "invalid email or password");
      return;
    }
    const token = crypto.randomUUID();
    sessions.set(token, {
      id: crypto.randomUUID(),
      name: "Comprador",
      email: body.email,
      roles: ["buyer"],
      created_at: "2026-01-01T00:00:00Z",
      sellerId: null,
      sellerName: null,
    });
    json(response, 200, currentUser({ token, ...sessions.get(token) }), {
      "set-cookie": `mercado_session=${token}; Path=/; HttpOnly; SameSite=Lax`,
    });
    return;
  }

  if (request.method === "POST" && url.pathname === "/v1/auth/logout") {
    const session = sessionOf(request);
    if (session) {
      sessions.delete(session.token);
    }
    response.writeHead(204, { "set-cookie": "mercado_session=; Path=/; HttpOnly; SameSite=Lax; Max-Age=0" });
    response.end();
    return;
  }

  if (request.method === "GET" && url.pathname === "/v1/auth/me") {
    const session = sessionOf(request);
    if (!session) {
      error(response, 401, "unauthenticated", "authentication required");
      return;
    }
    json(response, 200, currentUser(session));
    return;
  }

  if (request.method === "POST" && url.pathname === "/v1/seller/onboarding") {
    const session = sessionOf(request);
    if (!session) {
      error(response, 401, "unauthenticated", "authentication required");
      return;
    }
    const body = await readBody(request);
    const stored = sessions.get(session.token);
    stored.roles = ["buyer", "seller"];
    stored.sellerId = "01900000-0000-7000-8000-0000000000d1";
    stored.sellerName = body.display_name;
    json(response, 201, {
      id: stored.sellerId,
      user_id: stored.id,
      display_name: body.display_name,
      document: body.document,
      contact: body.contact,
      created_at: "2026-01-01T00:00:00Z",
    });
    return;
  }

  if (request.method === "GET" && url.pathname === "/v1/seller/listings") {
    const session = sessionOf(request);
    if (!session) {
      error(response, 401, "unauthenticated", "authentication required");
      return;
    }
    if (!session.roles.includes("seller")) {
      error(response, 403, "forbidden", "forbidden");
      return;
    }
    json(response, 200, catalog.filter((item) => item.created && item.seller.id === session.sellerId).map(sellerView));
    return;
  }

  if (request.method === "POST" && url.pathname === "/v1/seller/listings") {
    const session = sessionOf(request);
    if (!session || !session.roles.includes("seller")) {
      error(response, session ? 403 : 401, session ? "forbidden" : "unauthenticated", "forbidden");
      return;
    }
    const body = await readBody(request);
    const listing = {
      id: crypto.randomUUID(),
      title: body.title,
      description: body.description,
      price_cents: body.price_cents,
      currency: "BRL",
      condition: body.condition,
      free_shipping: Boolean(body.free_shipping),
      stock: body.stock,
      status: "draft",
      rejection_reason: null,
      category: categoryById(body.category_id),
      seller: { id: session.sellerId, display_name: session.sellerName },
      images: [],
      submitted_at: null,
      published_at: null,
      created_at: new Date().toISOString(),
      updated_at: new Date().toISOString(),
      created: true,
    };
    catalog.push(listing);
    json(response, 201, sellerView(listing));
    return;
  }

  if (url.pathname.startsWith("/v1/seller/listings/")) {
    const session = sessionOf(request);
    if (!session || !session.roles.includes("seller")) {
      error(response, session ? 403 : 401, session ? "forbidden" : "unauthenticated", "forbidden");
      return;
    }
    const parts = url.pathname.split("/").filter(Boolean);
    const listingId = parts[3];
    const action = parts[4];

    if (request.method === "GET" && !action) {
      const listing = sellerListing(response, session, listingId);
      if (listing) {
        json(response, 200, sellerView(listing));
      }
      return;
    }

    if (request.method === "PATCH" && !action) {
      const listing = sellerListing(response, session, listingId);
      if (!listing) {
        return;
      }
      const body = await readBody(request);
      const editable = listing.status === "draft" || listing.status === "rejected";
      if (editable) {
        if (body.title !== undefined) listing.title = body.title;
        if (body.description !== undefined) listing.description = body.description;
        if (body.category_id !== undefined) listing.category = categoryById(body.category_id);
        if (body.condition !== undefined) listing.condition = body.condition;
        if (body.free_shipping !== undefined) listing.free_shipping = body.free_shipping;
      }
      if (body.price_cents !== undefined) listing.price_cents = body.price_cents;
      listing.updated_at = new Date().toISOString();
      json(response, 200, sellerView(listing));
      return;
    }

    if (request.method === "POST" && action === "submit") {
      const listing = sellerListing(response, session, listingId);
      if (!listing) {
        return;
      }
      listing.status = "pending_review";
      listing.rejection_reason = null;
      listing.submitted_at = new Date().toISOString();
      json(response, 200, sellerView(listing));
      return;
    }

    if (request.method === "POST" && action === "archive") {
      const listing = sellerListing(response, session, listingId);
      if (!listing) {
        return;
      }
      listing.status = "archived";
      json(response, 200, sellerView(listing));
      return;
    }

    if (request.method === "POST" && action === "images") {
      const listing = sellerListing(response, session, listingId);
      if (!listing) {
        return;
      }
      await readRaw(request);
      if (listing.images.length >= 8) {
        error(response, 409, "image_limit_reached", "listing already has 8 images");
        return;
      }
      const image = { id: crypto.randomUUID(), url: `/v1/media/${listing.id}/${crypto.randomUUID()}.png`, position: listing.images.length };
      listing.images.push(image);
      json(response, 201, image);
      return;
    }

    if (request.method === "PATCH" && action === "images") {
      const listing = sellerListing(response, session, listingId);
      if (!listing) {
        return;
      }
      const body = await readBody(request);
      const reordered = (body.image_ids ?? []).map((id) => listing.images.find((image) => image.id === id)).filter(Boolean);
      if (reordered.length !== listing.images.length) {
        error(response, 400, "invalid_image_order", "image_ids must match the listing images");
        return;
      }
      listing.images = reordered;
      compaction(listing);
      json(response, 200, listing.images);
      return;
    }

    if (request.method === "DELETE" && action === "images") {
      const listing = sellerListing(response, session, listingId);
      if (!listing) {
        return;
      }
      const imageId = parts[5];
      listing.images = listing.images.filter((image) => image.id !== imageId);
      compaction(listing);
      response.writeHead(204);
      response.end();
      return;
    }

    if (request.method === "GET" && action === "inventory") {
      const listing = sellerListing(response, session, listingId);
      if (!listing) {
        return;
      }
      json(response, 200, inventory.get(listingId) ?? []);
      return;
    }

    if (request.method === "POST" && action === "inventory") {
      const listing = sellerListing(response, session, listingId);
      if (!listing) {
        return;
      }
      const body = await readBody(request);
      const quantity = listing.stock + Number(body.delta);
      if (quantity < 0) {
        error(response, 409, "insufficient_stock", "stock cannot become negative");
        return;
      }
      listing.stock = quantity;
      const adjustment = {
        id: crypto.randomUUID(),
        listing_id: listingId,
        actor_id: session.id,
        delta: Number(body.delta),
        quantity,
        reason: body.reason,
        created_at: new Date().toISOString(),
      };
      inventory.set(listingId, [adjustment, ...(inventory.get(listingId) ?? [])]);
      json(response, 200, adjustment);
      return;
    }

    error(response, 404, "not_found", "not found");
    return;
  }

  if (url.pathname.startsWith("/v1/admin/listings/")) {
    const parts = url.pathname.split("/").filter(Boolean);
    const listingId = parts[3];
    const action = parts[4];
    const listing = catalog.find((item) => item.id === listingId);
    if (!listing) {
      error(response, 404, "not_found", "listing not found");
      return;
    }
    const body = await readBody(request);
    if (action === "publish") {
      listing.status = "published";
      listing.published_at = new Date().toISOString();
    } else if (action === "reject") {
      listing.status = "rejected";
      listing.rejection_reason = body.reason;
    } else if (action === "suspend") {
      listing.status = "suspended";
    } else {
      error(response, 404, "not_found", "not found");
      return;
    }
    json(response, 200, sellerView(listing));
    return;
  }

  if (request.method === "GET" && url.pathname === "/v1/categories") {
    json(response, 200, categories);
    return;
  }

  if (request.method === "GET" && url.pathname === "/v1/catalog/listings") {
    sendCatalog(response, url);
    return;
  }

  if (request.method === "GET" && url.pathname.startsWith("/v1/listings/")) {
    const id = url.pathname.split("/").pop();
    const listing = catalog.find((item) => item.id === id && item.status === "published");
    if (!listing) {
      error(response, 404, "not_found", "listing not found");
      return;
    }
    const { created, ...rest } = listing;
    json(response, 200, rest);
    return;
  }

  if (request.method === "GET" && url.pathname === "/v1/status") {
    json(response, 200, { service: "mercado-api", version: "dev", commit: "none", status: "ok" });
    return;
  }

  response.writeHead(404);
  response.end();
});

server.listen(18082, "127.0.0.1");
