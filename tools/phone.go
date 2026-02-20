package tools

import (
	"fmt"
	"strings"
	"unicode"
)

// NormalizeWhatsAppTo normaliza um telefone para o formato aceito pelo WhatsApp Cloud API
// (apenas dígitos, em formato internacional, sem '+').
//
// Heurística atual (Brasil):
// - remove tudo que não é dígito
// - se vier com 10/11 dígitos, assume BR e prefixa 55
// - se já vier com DDI (>= 12 dígitos), mantém
func NormalizeWhatsAppTo(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("empty phone")
	}

	// mantém apenas dígitos
	var b strings.Builder
	b.Grow(len(raw))
	for _, r := range raw {
		if unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	phone := b.String()
	phone = strings.TrimLeft(phone, "0")

	// BR comum (DDD+numero): 10 ou 11 dígitos -> prefixa 55
	if len(phone) == 10 || len(phone) == 11 {
		phone = "55" + phone
	}

	// validação bem leve: DDI + número
	if len(phone) < 12 {
		return "", fmt.Errorf("invalid phone length: %d", len(phone))
	}
	return phone, nil
}
