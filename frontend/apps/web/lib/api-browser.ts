"use client";

import { createClient } from "@mercado/api-client";

export const browserApi = () =>
  createClient({
    baseUrl: "/api",
  });
