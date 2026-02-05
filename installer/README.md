# Instalador Penélope Chatbot (Apache + Postgres + Go)

Este repositório contém o instalador **dentro do próprio backend**, em `installer/`.

## Objetivo: 1 arquivo de configuração (JSON) como fonte única

Você mantém **um único arquivo sensível** `config.json` **localmente** (fora do git).
O `run.sh` **local** copia esse JSON para o servidor em:

- `/etc/penelope/config.json` (root:root, 600)

Depois disso, o instalador no servidor (`installer/install.sh`) lê esse JSON e:

- gera `/etc/penelope/runtime.config.json` (no formato que o backend Go já entende)
- gera `/etc/penelope/api.env` automaticamente (o backend continua lendo `.env`, mas você **não edita**)
- roda bootstrap (dependências, postgres, apache, systemd)
- roda deploy (clone/build/restart)
- roda certbot (opcional)

## Rodando no servidor (manual)

```bash
sudo bash installer/install.sh --config /etc/penelope/config.json --all
```

## Arquivos no servidor

- Config única (sensível): `/etc/penelope/config.json`
- Config gerada (sensível): `/etc/penelope/runtime.config.json`
- Env gerado (sensível): `/etc/penelope/api.env`
- Código (clonado pelo deploy): `/opt/penelope/api/src`
- Binário: `/opt/penelope/api/bin/penelope-api`
- Logs:
  - `/var/log/penelope/api.out.log`
  - `/var/log/penelope/api.err.log`

## Observações de segurança

- Não coloque o `config.json` em pastas servidas pela web.
- Evite `set -x` nos scripts para não vazar secrets em logs.
- Os arquivos em `/etc/penelope` são criados com permissão root-only.
