# Frontend Dockerfile
# Multi-stage build for optimized production image

# Stage 1: Build
FROM node:20-alpine AS builder
WORKDIR /app

# Install dependencies
COPY package*.json ./
RUN npm ci

# Copy source code
COPY . .

# Build the application
RUN npm run build

# Stage 2: Runtime
#
# nginx rather than `serve`, because this container has to do TWO things: serve
# the SPA and proxy /api to the backend on the same origin. `serve -s dist` only
# does the first, so every API call the browser made came back as index.html and
# signing up or signing in was impossible (#647). `serve` has no proxy mode, so
# this is a change of server, not of flags.
FROM nginx:1.27-alpine

# Replaces the packaged default server block, which listens on 80 and knows
# nothing about /api.
COPY nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=builder /app/dist /usr/share/nginx/html

EXPOSE 5173

# For hot-reload during development, run the dev server instead — vite.config.ts
# carries the same proxy for `npm run dev`.
CMD ["nginx", "-g", "daemon off;"]
