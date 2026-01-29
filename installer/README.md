# Instalador Penélope Chatbot (Apache + Postgres + Go)

Este diretório é **pra você manter localmente** e rodar sempre que quiser subir uma máquina nova.
Ele faz:

- Instala: Go (versão fixa), PostgreSQL, Apache2, Certbot
- Configura Postgres: cria DB + user + senha
- Configura Apache: VirtualHost `DOMINIO.conf` com proxy:
  - `/api` -> `127.0.0.1:5000`
  - `/admin` -> `127.0.0.1:8888` (placeholder por enquanto)
- Configura systemd:
  - `penelope-api.service` (no primeiro boot roda `go run .`, depois o deploy troca para binário)
  - `penelope-admin.service` (placeholder python http server)
- Deploy: clona/builda o repo `filiponegrao/penelope_chatbot_backend` e sobe o serviço.

## Como rodar

1) Ajuste o DNS na DigitalOcean:
- A record do `vendittoapp.com` (e/ou `api.vendittoapp.com` se preferir) apontando para o IP do servidor.

2) Execute:

```bash
chmod +x penelope-up.sh
./penelope-up.sh --host SEU_IP --domain vendittoapp.com --email admin@vendittoapp.com
```

### Se o clone pedir credenciais

Opção A (mais simples): Personal Access Token

```bash
export GITHUB_TOKEN="ghp_..."
./penelope-up.sh --host SEU_IP --domain vendittoapp.com --email admin@vendittoapp.com
```

Opção B: Deploy key (base64)

```bash
export GIT_SSH_KEY_B64="$(base64 -w0 ~/.ssh/id_ed25519)"
./penelope-up.sh --host SEU_IP --domain vendittoapp.com --email admin@vendittoapp.com
```

## Onde ficam as configs no servidor

- Env do runtime: `/etc/penelope/api.env`
- Config do backend: `/etc/penelope/config.json`
- Código: `/opt/penelope/api/src`
- Binário: `/opt/penelope/api/bin/penelope-api`
- Logs:
  - `/var/log/penelope/api.out.log`
  - `/var/log/penelope/api.err.log`

## Observação importante (paths)

Seu backend hoje expõe rotas em `/api/...` (via group "/api" no router).
O Apache está proxyando `/api` para `http://127.0.0.1:5000/api` (mantendo o prefixo).

## Admin

Por enquanto o `/admin` é placeholder (python http server).
Quando você criar o admin real (provavelmente outro binário / outro repo),
a única coisa que muda é o `penelope-admin.service` e o target do ProxyPass.

