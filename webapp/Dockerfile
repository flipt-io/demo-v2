FROM node:22-alpine AS builder
WORKDIR /app
COPY package*.json ./
RUN npm i
COPY . .
RUN npm run build

FROM nginxinc/nginx-unprivileged:1.29.0-alpine3.22-otel
USER 0
COPY --from=builder /app/dist /usr/share/nginx/html
RUN sed -i "1i load_module modules/ngx_otel_module.so;" /etc/nginx/nginx.conf
COPY nginx-default.conf /etc/nginx/conf.d/default.conf
USER 101
EXPOSE 4000 
CMD ["nginx", "-g", "daemon off;"]
