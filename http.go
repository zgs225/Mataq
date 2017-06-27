package maatq

type response struct {
	Ok      bool   `json:"ok"`
	Code    int    `json:"code"`
	EventId string `json:"event_id"`
	Err     string `json:err"`
}

type delayRequest struct {
	Event string      `json:"event"`
	Data  interface{} `json:"data"`
	Delay string      `json:"delay"`
}

type periodRequest struct {
	Event  string      `json:"event"`
	Data   interface{} `json:"data"`
	Period int64       `json:"period"`
}

type crontabRequest struct {
	Event   string      `json:"event"`
	Data    interface{} `json:"data"`
	Crontab string      `json:"crontab"`
}
