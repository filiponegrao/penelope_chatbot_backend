<VirtualHost *:80>
    ServerName {{DOMAIN}}
    DocumentRoot /var/www/penelope/empty

    <Directory "/var/www/penelope/empty">
        Require all granted
        Options -Indexes
        AllowOverride None
    </Directory>

    ProxyPreserveHost On

    # Páginas estáticas (HTML) servidas por URLs "*.pdf" por conveniência.
    # Observação: o conteúdo entregue é HTML (não PDF).
    Alias /policy.pdf /var/www/penelope/empty/policy.html
    Alias /terms.pdf  /var/www/penelope/empty/terms.html
    # Alternativas sem extensão
    Alias /policy     /var/www/penelope/empty/policy.html
    Alias /terms      /var/www/penelope/empty/terms.html

    # / -> não faz nada por enquanto (servindo um index.html vazio)
    # /api -> API (backend escuta em /)
    ProxyPass        /api  http://127.0.0.1:{{API_PORT}}/api retry=0
    ProxyPassReverse /api  http://127.0.0.1:{{API_PORT}}/api

    # Redireciona tudo pra HTTPS (exceto ACME challenge do certbot)
    RewriteEngine On
    RewriteCond %{REQUEST_URI} !^/\.well-known/acme-challenge/
    RewriteCond %{HTTPS} !=on
    RewriteRule ^ https://%{HTTP_HOST}%{REQUEST_URI} [L,R=301]
</VirtualHost>
