FROM node:24-alpine AS build
RUN corepack enable
WORKDIR /workspace
COPY pnpm-workspace.yaml package.json pnpm-lock.yaml .npmrc tsconfig.base.json turbo.json ./
COPY packages ./packages
COPY apps/storefront-web ./apps/storefront-web
RUN pnpm install --frozen-lockfile --filter @mall/storefront-web...
RUN pnpm --filter @mall/storefront-web build

FROM node:24-alpine
WORKDIR /app
ENV NODE_ENV=production
ENV HOST=0.0.0.0
ENV PORT=3000
COPY --from=build /workspace/apps/storefront-web/.output ./.output
EXPOSE 3000
CMD ["node", ".output/server/index.mjs"]
