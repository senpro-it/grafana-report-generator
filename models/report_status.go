package models

type Reportstatus struct {
	ReportId      int     `json:"report_id"`
	Progress      int     `json:"progress"`
	Status        string  `json:"status"`
	Done          bool    `json:"done"`
	ExecutionTime float64 `json:"execution_time"`
}
