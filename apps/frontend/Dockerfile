FROM node:22-alpine AS builder

WORKDIR /app

COPY package.json package-lock.json ./
RUN npm ci

COPY . .

ARG VITE_API_BASE_URL=http://localhost:8080
ARG VITE_APP_VERSION=container-local
ARG VITE_IMAGE_GENERATION_ENABLED=false
ENV VITE_API_BASE_URL=$VITE_API_BASE_URL
ENV VITE_APP_VERSION=$VITE_APP_VERSION
ENV VITE_IMAGE_GENERATION_ENABLED=$VITE_IMAGE_GENERATION_ENABLED

RUN npm run build

FROM node:22-alpine

WORKDIR /app

RUN npm install -g serve

COPY --from=builder /app/dist ./dist
COPY serve.json ./serve.json

ENV PORT=3000

EXPOSE 3000

CMD ["sh", "-c", "serve -s dist -c /app/serve.json -l ${PORT:-3000}"]
