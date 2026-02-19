# /etc/apache2/conf-available/penelope-webhook-mtls.conf
# Habilita mTLS somente para o endpoint do webhook.
# Requer que o arquivo de CA exista em: /etc/ssl/certs/meta-webhooks-root-ca.pem

<IfModule mod_ssl.c>
    SSLCACertificateFile /etc/ssl/certs/meta-webhooks-root-ca.pem

    # Exige certificado cliente APENAS no webhook (inclui /api/webhook e /api/webhook/123)
    <LocationMatch "^/api/webhook">
        SSLVerifyClient require
        SSLVerifyDepth 3
    </LocationMatch>
</IfModule>
