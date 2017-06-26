package maatq

import (
	"encoding/json"
	"errors"
	"reflect"
	"runtime"
	"strconv"
)

func AtoString(val interface{}) (string, error) {
	sval, ok := val.(string)
	if ok {
		return sval, nil
	}

	fval, ok := val.(float64)
	if ok {
		return strconv.FormatInt(int64(fval), 10), nil
	}

	bval, ok := val.(bool)
	if ok {
		return strconv.FormatBool(bval), nil
	}

	jval, ok := val.(map[string]interface{})
	if ok {
		tmpVal, err := json.Marshal(jval)
		if err == nil {
			return string(tmpVal), nil
		}
		return "", err
	}

	return "", errors.New("Unknown type")
}

func GetFunctionName(i interface{}) string {
	return runtime.FuncForPC(reflect.ValueOf(i).Pointer()).Name()
}

func makeRangeOfInt8(min, max int8, step int) []int8 {
	s := make([]int8, 0)
	for i := min; i <= max; i += int8(step) {
		s = append(s, i)
	}
	return s
}
