import { NextResponse, type NextRequest } from "next/server";

const apiBaseUrl = process.env.API_BASE_URL ?? "http://127.0.0.1:8080";

export function middleware(request: NextRequest) {
  const target = new URL(request.nextUrl.pathname.replace(/^\/api/, "") + request.nextUrl.search, apiBaseUrl);
  return NextResponse.rewrite(target);
}

export const config = {
  matcher: "/api/:path*",
};
