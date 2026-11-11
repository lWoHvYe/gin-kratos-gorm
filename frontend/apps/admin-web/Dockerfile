FROM node:24-alpine AS build
RUN corepack enable
WORKDIR /workspace
COPY pnpm-workspace.yaml package.json pnpm-lock.yaml .npmrc tsconfig.base.json turbo.json ./
COPY packages ./packages
COPY apps/admin-web ./apps/admin-web
RUN pnpm install --frozen-lockfile --filter @mall/admin-web...
RUN pnpm --filter @mall/admin-web build

FROM nginx:1.29-alpine
COPY apps/admin-web/nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /workspace/apps/admin-web/dist /usr/share/nginx/html/admin
EXPOSE 8080
