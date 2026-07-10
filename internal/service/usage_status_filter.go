package service

import "gorm.io/gorm"

const (
	UsageStatusSuccess = "success"
	UsageStatusError   = "error"
)

func IsValidUsageStatus(status string) bool {
	return status == "" || status == UsageStatusSuccess || status == UsageStatusError
}

func applyUsageStatusFilter(query *gorm.DB, status string, statusCode int) *gorm.DB {
	switch status {
	case UsageStatusSuccess:
		return query.Where("status_code < ?", 400)
	case UsageStatusError:
		return query.Where("status_code >= ?", 400)
	default:
		if statusCode > 0 {
			return query.Where("status_code = ?", statusCode)
		}
		return query
	}
}
