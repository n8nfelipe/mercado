FROM node:24-alpine AS base
ENV PNPM_HOME=/pnpm
ENV PATH=$PNPM_HOME:$PATH
RUN corepack enable
WORKDIR /app

FROM base AS deps
COPY package.json pnpm-lock.yaml pnpm-workspace.yaml ./
COPY frontend/packages/api-client/package.json frontend/packages/api-client/package.json
COPY frontend/packages/ui/package.json frontend/packages/ui/package.json
COPY frontend/apps/web/package.json frontend/apps/web/package.json
RUN pnpm install --frozen-lockfile

FROM deps AS builder
COPY . .
RUN pnpm --filter @mercado/web... build

FROM node:24-alpine AS runner
WORKDIR /app
ENV NODE_ENV=production
ENV PORT=3000
ENV HOSTNAME=0.0.0.0
RUN addgroup -S nodejs && adduser -S nextjs -G nodejs
COPY --from=builder --chown=nextjs:nodejs /app/frontend/apps/web/.next/standalone ./
COPY --from=builder --chown=nextjs:nodejs /app/frontend/apps/web/.next/static ./frontend/apps/web/.next/static
COPY --from=builder --chown=nextjs:nodejs /app/frontend/apps/web/public ./frontend/apps/web/public
USER nextjs
EXPOSE 3000
HEALTHCHECK --interval=30s --timeout=3s --start-period=15s --retries=3 \
  CMD wget -qO- http://127.0.0.1:3000/healthz >/dev/null || exit 1
CMD ["node", "frontend/apps/web/server.js"]
