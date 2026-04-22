FROM node:22-alpine AS builder

WORKDIR /src
COPY package.json package-lock.json* ./
RUN npm install
COPY . .
ARG VITE_API_BASE_URL=""
ENV VITE_API_BASE_URL=${VITE_API_BASE_URL}
RUN npm run build

FROM nginx:alpine

COPY --from=builder /src/dist /usr/share/nginx/html
RUN rm -f /etc/nginx/conf.d/*
COPY nginx.conf /etc/nginx/conf.d/default.conf

EXPOSE 5173
