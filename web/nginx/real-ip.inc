# Client address when Plys runs behind another reverse proxy (Caddy, Traefik, ...): requests coming from these
# addresses may set X-Forwarded-For, and nginx (logs, forwarded header) uses the right-most address outside them.
# Private ranges cover a proxy in the same compose project or on the Docker host. Replace this file (mount it at
# /etc/nginx/snippets/real-ip.inc) to trust only your proxy, or a proxy reached through a public address.
set_real_ip_from 127.0.0.0/8;
set_real_ip_from 10.0.0.0/8;
set_real_ip_from 172.16.0.0/12;
set_real_ip_from 192.168.0.0/16;
set_real_ip_from ::1/128;
set_real_ip_from fc00::/7;
real_ip_header X-Forwarded-For;
real_ip_recursive on;
