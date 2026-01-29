[Unit]
Description=Penelope Chatbot API
After=network.target postgresql.service
Wants=postgresql.service

[Service]
Type=simple
User={{APP_USER}}
Group={{APP_USER}}
WorkingDirectory={{API_DIR}}/src
EnvironmentFile={{API_ENV}}

# Primeiro deploy roda com "go run ." até existir binário,
# depois o deploy troca para ExecStart=/opt/penelope/api/bin/penelope-api
ExecStart={{GO_BIN}} run .
Restart=always
RestartSec=3

StandardOutput=append:/var/log/penelope/api.out.log
StandardError=append:/var/log/penelope/api.err.log

NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/var/log/penelope

[Install]
WantedBy=multi-user.target
