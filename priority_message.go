package maatq

import (
	"encoding/json"
	"time"

	log "github.com/Sirupsen/logrus"
)

// 根据时间调度的消息
type PriorityMessage struct {
	Message
	T int64      `json:"t"` // 下一次执行的时间
	P Periodicor `json:"p"` // 周期
}

func (pm *PriorityMessage) IsDue() bool {
	return time.Now().Unix() >= pm.T
}

func (pm *PriorityMessage) IsPeriodic() bool {
	return pm.P != nil
}

func (pm *PriorityMessage) ToLogFields() log.Fields {
	v := (&pm.Message).ToLogFields()
	v["t"] = pm.T
	return v
}

func (pm *PriorityMessage) String() string {
	b, _ := json.Marshal(pm)
	return string(b)
}
