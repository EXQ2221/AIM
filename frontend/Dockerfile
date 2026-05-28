ARG NODE_IMAGE=m.daocloud.io/docker.io/library/node:20-alpine
ARG NGINX_IMAGE=m.daocloud.io/docker.io/library/nginx:1.27-alpine

FROM ${NODE_IMAGE} AS build
WORKDIR /app

COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci --no-audit --no-fund

COPY frontend/ ./
RUN npm run build

FROM ${NGINX_IMAGE}
COPY frontend/nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /app/dist /usr/share/nginx/html
