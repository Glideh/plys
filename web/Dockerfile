# syntax=docker/dockerfile:1
ARG NODE_VERSION=24

FROM node:${NODE_VERSION}-alpine AS dev
# Named volume inherits these permissions so any host UID can install modules.
RUN mkdir -p /app/node_modules && chmod 777 /app/node_modules
ENV npm_config_cache=/tmp/.npm \
    HOME=/tmp
WORKDIR /app
EXPOSE 5173

FROM node:${NODE_VERSION}-alpine AS build
WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci
COPY . .
RUN npm run build

FROM nginxinc/nginx-unprivileged:1.29-alpine AS prod
COPY nginx/default.conf /etc/nginx/conf.d/default.conf
COPY nginx/security-headers.inc /etc/nginx/snippets/security-headers.inc
COPY --from=build /app/dist /usr/share/nginx/html
EXPOSE 8080
