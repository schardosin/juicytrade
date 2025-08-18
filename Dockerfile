# Multi-stage Dockerfile to build Vue frontend and run Python backend with nginx + supervisord
# Stage 1: Build frontend
FROM node:20-alpine AS frontend-builder
WORKDIR /app
# Install dependencies
COPY trade-app/package.json trade-app/package-lock.json ./trade-app/
RUN cd trade-app && npm ci --silent

# Copy frontend sources and build
COPY trade-app ./trade-app
WORKDIR /app/trade-app
RUN npm run build

# Stage 2: Final image with Python, nginx and supervisor
FROM python:3.11-slim

ENV PYTHONUNBUFFERED=1 \
    PATH=/root/.local/bin:$PATH

# Install system deps (nginx, supervisor, curl)
RUN apt-get update && apt-get install -y --no-install-recommends \
    nginx \
    supervisor \
    curl \
    build-essential \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# Create app directory
WORKDIR /app

# Copy backend source and requirements
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

# Copy the backend code
COPY trade_backend ./trade_backend

# Copy built frontend from builder (install into both common nginx document roots)
COPY --from=frontend-builder /app/trade-app/dist /usr/share/nginx/html
COPY --from=frontend-builder /app/trade-app/dist /var/www/html
# Remove default nginx welcome page if present
RUN rm -f /var/www/html/index.nginx-debian.html || true

# Remove default nginx site and replace with our config
RUN rm -f /etc/nginx/sites-enabled/default
COPY docker/nginx.conf /etc/nginx/conf.d/default.conf

# Copy supervisord config
COPY docker/supervisord.conf /etc/supervisor/supervisord.conf

# Expose frontend port
EXPOSE 80
# Optional: expose backend port for debugging (commented out by default)
# EXPOSE 8008

# Create data directory structure for configuration and cache files
RUN mkdir -p /app/data/config /app/data/cache

# Ensure nginx can read files and supervisor has proper dirs
RUN mkdir -p /var/log/supervisor && chown -R www-data:www-data /usr/share/nginx/html

# Entrypoint - run supervisord in foreground (PID 1)
CMD ["/usr/bin/supervisord", "-n", "-c", "/etc/supervisor/supervisord.conf"]
