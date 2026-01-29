<VirtualHost *:80>
    ServerName {{DOMAIN}}

    # Proxy settings
    ProxyPreserveHost On
    RequestHeader set X-Forwarded-Proto "http"
    RequestHeader set X-Forwarded-Port "80"

    # /api -> Go backend em localhost:{{API_PORT}}
    ProxyPass        "/api"  "http://127.0.0.1:{{API_PORT}}/api" retry=0
    ProxyPassReverse "/api"  "http://127.0.0.1:{{API_PORT}}/api"

    # /admin -> admin em localhost:{{ADMIN_PORT}}
    ProxyPass        "/admin"  "http://127.0.0.1:{{ADMIN_PORT}}/" retry=0
    ProxyPassReverse "/admin"  "http://127.0.0.1:{{ADMIN_PORT}}/"

    ErrorLog  /var/log/apache2/{{DOMAIN}}-error.log
    CustomLog /var/log/apache2/{{DOMAIN}}-access.log combined
</VirtualHost>
