#!/bin/bash
# Run ON the control plane VM as root. Moves Warmbly from IP:port URLs to
# https://portal.fullpilot.com (+ admin/api/ws/track/forms subdomains) behind
# a Caddy container that issues Let's Encrypt certificates automatically.
# Idempotent: safe to re-run.
set -euo pipefail
cd /opt/warmbly

HOST=portal.fullpilot.com
ACME_EMAIL=brandon.hays@fullpilot.com

cp .env ".env.bak.$(date +%Y%m%d%H%M%S)"

setenv() { # setenv KEY VALUE  -> replace or append in .env
  if grep -qE "^$1=" .env; then
    sed -i "s|^$1=.*|$1=$2|" .env
  else
    echo "$1=$2" >> .env
  fi
}

setenv PUBLIC_HOST        "$HOST"
setenv APP_URL            "https://$HOST"
setenv API_PUBLIC_URL     "https://api.$HOST"
setenv CORS_ALLOW_ORIGINS "https://$HOST,https://admin.$HOST"
setenv WEBSOCKET_URL      "wss://ws.$HOST/socket/websocket"
setenv PHX_HOST           "ws.$HOST"
setenv CHECK_ORIGIN       "true"
setenv TRACKING_DOMAIN    "track.$HOST"
setenv FORMS_DOMAIN       "forms.$HOST"
setenv TRUSTED_PROXIES    "172.16.0.0/12,10.0.0.0/8,192.168.0.0/16"
setenv EMAIL_ADDRESS      "noreply@fullpilot.com"

cat > Caddyfile <<EOF
# Warmbly behind automatic HTTPS. Managed by infra/switch-to-domain.sh.
{
	email $ACME_EMAIL
}

$HOST {
	reverse_proxy web:80
}

admin.$HOST {
	reverse_proxy admin:80
}

api.$HOST {
	reverse_proxy backend:8080
}

ws.$HOST {
	reverse_proxy realtime:4000
}

track.$HOST {
	reverse_proxy tracking:3000
}

forms.$HOST {
	reverse_proxy forms:8090
}
EOF

cat > docker-compose.override.yml <<'EOF'
# Fullpilot additions. Managed by infra/switch-to-domain.sh.
#  - NATS and Redis published on the internal VPC address only, for the
#    Cloud Run workers (Direct VPC egress). Never on the public IP.
#  - Caddy terminates TLS for portal.fullpilot.com and its subdomains and
#    proxies to the services over the compose network.
services:
  nats:
    ports:
      - "10.128.0.2:4222:4222"
  redis:
    ports:
      - "10.128.0.2:6379:6379"
  caddy:
    image: caddy:2-alpine
    restart: unless-stopped
    ports:
      - "80:80"
      - "443:443"
      - "443:443/udp"
    volumes:
      - ./Caddyfile:/etc/caddy/Caddyfile:ro
      - caddy_data:/data
      - caddy_config:/config
    depends_on:
      backend: { condition: service_healthy }

volumes:
  caddy_data:
  caddy_config:
EOF

docker compose -p warmbly up -d --remove-orphans
echo "waiting for services..."
sleep 25
docker compose -p warmbly ps --format 'table {{.Name}}\t{{.Status}}'
echo "--- caddy log (last 15) ---"
docker compose -p warmbly logs caddy --tail 15 --no-log-prefix 2>&1 | cut -c1-200
