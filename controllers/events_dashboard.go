package controllers

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	dbpkg "penelope/db"
	"penelope/models"

	"github.com/gin-gonic/gin"
)

// ------------------------------
// Dashboard - Stats
// ------------------------------

type processedPerDayRow struct {
	Day   string `json:"day"`
	Count int64  `json:"count"`
}

// GET /api/events/dashboard/processed-per-day
// Query params:
// - from=YYYY-MM-DD (optional, default: hoje-6)
// - to=YYYY-MM-DD   (optional, default: hoje)
// Retorna uma série diária (inclui dias com 0).
func GetEventsProcessedPerDay(c *gin.Context) {
	user, ok := GetUserLogged(c)
	if !ok {
		RespondError(c, "unauthorized", http.StatusUnauthorized)
		return
	}

	db := dbpkg.DBInstance(c)
	if db == nil {
		RespondError(c, "db não configurado no contexto", http.StatusInternalServerError)
		return
	}

	from, to, ok := parseDateRange(c)
	if !ok {
		return
	}

	// Normaliza para início/fim do dia.
	// Normaliza para início do dia e usa "to exclusivo" (dia seguinte 00:00).
	from = time.Date(from.Year(), from.Month(), from.Day(), 0, 0, 0, 0, time.Local)
	toInclusive := time.Date(to.Year(), to.Month(), to.Day(), 0, 0, 0, 0, time.Local)
	toExclusive := toInclusive.AddDate(0, 0, 1)

	// Monta a query dependendo do dialeto.
	dialect := strings.ToLower(db.Dialect().GetName())

	// timezone do "dia de negócio"
	tz := strings.TrimSpace(c.DefaultQuery("tz", "America/Sao_Paulo"))

	dayExpr := "date(processed_at)" // fallback genérico

	if strings.Contains(dialect, "sqlite") {
		// SQLite: força dia local
		dayExpr = "strftime('%Y-%m-%d', processed_at, 'localtime')"
	} else if strings.Contains(dialect, "postgres") {
		// Postgres: força dia no timezone escolhido
		// AT TIME ZONE converte o timestamptz para timestamp "local" naquele tz
		dayExpr = fmt.Sprintf("to_char(date_trunc('day', processed_at AT TIME ZONE '%s'), 'YYYY-MM-DD')", tz)
	}
	if strings.Contains(dialect, "postgres") {
		dayExpr = "to_char(date_trunc('day', processed_at), 'YYYY-MM-DD')"
	}

	var rows []processedPerDayRow
	q := db.Table("events").
		Select(fmt.Sprintf("%s as day, count(*) as count", dayExpr)).
		Where("user_id = ?", user.ID).
		Where("status = ? AND processed_at IS NOT NULL AND processed_at >= ? AND processed_at < ?",
			models.EVENT_STATUS_DONE, from, toExclusive).
		Group("day").
		Order("day asc")

	if err := q.Scan(&rows).Error; err != nil {
		RespondError(c, err.Error(), http.StatusBadRequest)
		return
	}

	// Preenche dias faltantes com 0.
	series := fillDailySeries(from, to, rows)
	RespondSuccess(c, gin.H{
		"from":   from.Format("2006-01-02"),
		"to":     to.Format("2006-01-02"),
		"series": series,
	})
}

type monthlyUsageResponse struct {
	Month     string `json:"month"`
	Used      int64  `json:"used"`
	Limit     int64  `json:"limit"`
	Remaining int64  `json:"remaining"`
}

// GET /api/events/dashboard/monthly-usage
// Query params:
// - month=YYYY-MM (optional, default: mês atual)
// Retorna o total de eventos processados no mês + limite do plano do usuário.
func GetEventsMonthlyUsage(c *gin.Context) {
	user, ok := GetUserLogged(c)
	if !ok {
		RespondError(c, "unauthorized", http.StatusUnauthorized)
		return
	}

	db := dbpkg.DBInstance(c)
	if db == nil {
		RespondError(c, "db não configurado no contexto", http.StatusInternalServerError)
		return
	}

	monthStart, monthEnd, monthLabel, ok := parseMonthRange(c)
	if !ok {
		return
	}

	// usage (POR USUÁRIO)
	var used int64
	if err := db.Model(&models.Event{}).
		Where("user_id = ?", user.ID).
		Where("status = ? AND processed_at IS NOT NULL AND processed_at >= ? AND processed_at < ?",
			models.EVENT_STATUS_DONE, monthStart, monthEnd).
		Count(&used).Error; err != nil {
		RespondError(c, err.Error(), http.StatusBadRequest)
		return
	}

	// limit (via plano do usuário)
	var limit int64
	planID, err := getUserPlanID(db, user.ID)
	if err != nil {
		RespondError(c, err.Error(), http.StatusBadRequest)
		return
	}
	if planID != nil {
		var plan models.Plan
		if err := db.First(&plan, *planID).Error; err == nil {
			limit = plan.MonthlyMessageLimit
		}
	}

	remaining := int64(0)
	if limit > 0 {
		remaining = limit - used
		if remaining < 0 {
			remaining = 0
		}
	}

	RespondSuccess(c, monthlyUsageResponse{
		Month:     monthLabel,
		Used:      used,
		Limit:     limit,
		Remaining: remaining,
	})
}

// ------------------------------
// Dashboard - List
// ------------------------------

// GET /api/events/dashboard/list
// Query params:
// - status=pending|processing|done|invalidated (optional)
// - q=texto (optional) -> busca em recipient + text + reply_text
// - sort_by=created_at|processed_at|scheduled_at|id (optional, default: created_at)
// - order=asc|desc (optional, default: desc)
// - limit (optional, default: 200, max: 500)
// - offset (optional, default: 0)
func GetEventsDashboardList(c *gin.Context) {
	user, ok := GetUserLogged(c)
	if !ok {
		RespondError(c, "unauthorized", http.StatusUnauthorized)
		return
	}

	db := dbpkg.DBInstance(c)
	if db == nil {
		RespondError(c, "db não configurado no contexto", http.StatusInternalServerError)
		return
	}

	status := strings.TrimSpace(c.Query("status"))
	q := strings.TrimSpace(c.Query("q"))
	sortBy := strings.TrimSpace(c.DefaultQuery("sort_by", "created_at"))
	order := strings.ToLower(strings.TrimSpace(c.DefaultQuery("order", "desc")))

	limit := clampInt(queryInt(c, "limit", 200), 1, 500)
	offset := clampInt(queryInt(c, "offset", 0), 0, 1_000_000)

	// whitelist sort fields
	switch sortBy {
	case "created_at", "processed_at", "scheduled_at", "id":
	default:
		sortBy = "created_at"
	}
	if order != "asc" {
		order = "desc"
	}

	// BASE POR USUÁRIO
	query := db.Model(&models.Event{}).Where("user_id = ?", user.ID)

	if status != "" {
		query = query.Where("status = ?", status)
	}
	if q != "" {
		like := "%%" + q + "%%"
		query = query.Where("recipient LIKE ? OR text LIKE ? OR reply_text LIKE ?", like, like, like)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		RespondError(c, err.Error(), http.StatusBadRequest)
		return
	}

	var events []models.Event
	if err := query.Order(fmt.Sprintf("%s %s", sortBy, order)).
		Limit(limit).
		Offset(offset).
		Find(&events).Error; err != nil {
		RespondError(c, err.Error(), http.StatusBadRequest)
		return
	}

	RespondSuccess(c, gin.H{
		"total":  total,
		"limit":  limit,
		"offset": offset,
		"events": events,
	})
}

// ------------------------------
// Helpers
// ------------------------------

func queryInt(c *gin.Context, key string, def int) int {
	v := strings.TrimSpace(c.Query(key))
	if v == "" {
		return def
	}
	var n int
	_, err := fmt.Sscanf(v, "%d", &n)
	if err != nil {
		return def
	}
	return n
}

func clampInt(v, min, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

func parseDateRange(c *gin.Context) (time.Time, time.Time, bool) {
	// defaults: últimos 7 dias
	now := time.Now()
	defTo := now
	defFrom := now.AddDate(0, 0, -6)

	fromStr := strings.TrimSpace(c.Query("from"))
	toStr := strings.TrimSpace(c.Query("to"))

	from := defFrom
	to := defTo
	var err error

	if fromStr != "" {
		from, err = time.ParseInLocation("2006-01-02", fromStr, time.Local)
		if err != nil {
			RespondError(c, "from inválido (use YYYY-MM-DD)", http.StatusBadRequest)
			return time.Time{}, time.Time{}, false
		}
	}
	if toStr != "" {
		to, err = time.ParseInLocation("2006-01-02", toStr, time.Local)
		if err != nil {
			RespondError(c, "to inválido (use YYYY-MM-DD)", http.StatusBadRequest)
			return time.Time{}, time.Time{}, false
		}
	}
	if from.After(to) {
		RespondError(c, "from não pode ser maior que to", http.StatusBadRequest)
		return time.Time{}, time.Time{}, false
	}
	return from, to, true
}

func parseMonthRange(c *gin.Context) (time.Time, time.Time, string, bool) {
	now := time.Now()
	month := strings.TrimSpace(c.Query("month"))
	if month == "" {
		month = now.Format("2006-01")
	}
	t, err := time.ParseInLocation("2006-01", month, time.Local)
	if err != nil {
		RespondError(c, "month inválido (use YYYY-MM)", http.StatusBadRequest)
		return time.Time{}, time.Time{}, "", false
	}
	start := time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.Local)
	end := start.AddDate(0, 1, 0)
	return start, end, month, true
}

func fillDailySeries(from time.Time, to time.Time, rows []processedPerDayRow) []processedPerDayRow {
	// mapa day->count
	m := map[string]int64{}
	for _, r := range rows {
		if r.Day == "" {
			continue
		}
		m[r.Day] = r.Count
	}

	var out []processedPerDayRow
	// itera por dia (inclusive)
	cur := time.Date(from.Year(), from.Month(), from.Day(), 0, 0, 0, 0, time.Local)
	end := time.Date(to.Year(), to.Month(), to.Day(), 0, 0, 0, 0, time.Local)
	for !cur.After(end) {
		key := cur.Format("2006-01-02")
		out = append(out, processedPerDayRow{Day: key, Count: m[key]})
		cur = cur.AddDate(0, 0, 1)
	}
	return out
}
