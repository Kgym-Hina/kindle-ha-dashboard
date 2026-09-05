FROM node:22-alpine AS web-build

WORKDIR /workspace/web
COPY web/package*.json ./
RUN npm install --no-audit --no-fund
COPY web/ ./
RUN npm run build

FROM node:22-alpine AS server-build

WORKDIR /workspace/server
COPY server/package*.json ./
RUN npm install --no-audit --no-fund
COPY server/ ./
RUN npm run build

FROM node:22-alpine

ENV NODE_ENV=production
ENV PORT=8099
WORKDIR /app
COPY --from=server-build /workspace/server/package*.json ./
COPY --from=server-build /workspace/server/node_modules ./node_modules
COPY --from=server-build /workspace/server/dist ./dist
COPY --from=web-build /workspace/web/dist ./public
RUN mkdir -p /data/documents
EXPOSE 8099
CMD ["node", "dist/server.js"]
