[Unit]
Description=Penelope Admin (placeholder)
After=network.target

[Service]
Type=simple
User={{APP_USER}}
Group={{APP_GROUP}}
WorkingDirectory={{ADMIN_DIR}}
# Placeholder: serve uma pasta estática /opt/penelope/admin/public em {{ADMIN_PORT}}
ExecStart=/usr/bin/python3 -m http.server {{ADMIN_PORT}}
Restart=always
RestartSec=3

StandardOutput=append:/var/log/penelope/admin.out.log
StandardError=append:/var/log/penelope/admin.err.log

NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=read-only
ReadWritePaths=/var/log/penelope {{ADMIN_DIR}}

[Install]
WantedBy=multi-user.target
