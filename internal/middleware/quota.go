package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/warm3snow/llm-gateway/internal/models"
	"github.com/warm3snow/llm-gateway/internal/service"
)

// authQuotaService is a process-wide singleton: the QuotaService is stateless
// apart from the DB handle, so one instance serves all requests.
var (
	authQuotaOnce sync.Once
	authQuotaSvc  *service.QuotaService
)

func newAuthQuotaService() *service.QuotaService {
	authQuotaOnce.Do(func() {
		authQuotaSvc = service.NewQuotaService()
	})
	return authQuotaSvc
}

// userPendingCost returns the pending (not yet flushed) spend attributable to
// the key's owning user. The budget tracker tracks per-virtual-key pending
// cost; a user's pending cost is the sum over keys we know about via the
// cache digest map is not feasible here, so the caller passes the current
// key's pending cost as an approximation until the async flush catches up.
func userPendingCost(vk *models.VirtualKey, owner *models.User, pendingBudget float64) (float64, bool) {
	if owner == nil {
		return 0, false
	}
	return pendingBudget, true
}

// tenantPendingCost approximates the tenant's pending spend with the current
// key's pending cost; the async flush keeps BudgetUsed authoritative.
func tenantPendingCost(vk *models.VirtualKey, pendingBudget float64) float64 {
	return pendingBudget
}

// requestedModelFromBody extracts the model from the request: the
// x-llm-model header first, then the buffered JSON body.
func requestedModelFromBody(c *gin.Context, body []byte) string {
	if m := c.GetHeader("x-llm-model"); m != "" {
		return m
	}
	if len(body) == 0 {
		return ""
	}
	return modelFromBody(body)
}

func modelFromBody(body []byte) string {
	var payload struct {
		Model string `json:"model"`
	}
	if err := json.NewDecoder(bytes.NewReader(body)).Decode(&payload); err != nil {
		return ""
	}
	return payload.Model
}

// readBodyOnce reads the request body (if any) so it can be inspected and
// replayed downstream. Returns nil when the body is absent.
func readBodyOnce(c *gin.Context) []byte {
	if c.Request.Body == nil {
		return nil
	}
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return nil
	}
	c.Request.Body = io.NopCloser(bytes.NewBuffer(body))
	return body
}
