# Build context = frontend/ : docker build -f frontend/Dockerfile frontend
FROM node:22-alpine AS build
WORKDIR /app
COPY package.json package-lock.json* ./
RUN npm install
COPY . .
RUN npm run build

FROM node:22-alpine AS run
WORKDIR /app
ENV NODE_ENV=production
RUN adduser -D -u 10001 giti
COPY --from=build /app/.next/standalone ./
COPY --from=build /app/.next/static ./.next/static
COPY --from=build /app/public ./public
USER giti
ENV PORT=8080 HOSTNAME=0.0.0.0
EXPOSE 8080
HEALTHCHECK --interval=10s --timeout=3s --retries=5 CMD wget -qO- http://127.0.0.1:8080/en/login >/dev/null 2>&1 || exit 1
CMD ["node", "server.js"]
