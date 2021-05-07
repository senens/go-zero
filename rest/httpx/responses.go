package httpx

import (
	"encoding/json"
	"net/http"
	"reflect"
	"sync"
	"time"

	"github.com/tal-tech/go-zero/core/logx"
)

var (
	errorHandler func(error) (int, interface{})
	lock         sync.RWMutex
)

type RespJsonStruct struct {
	Code int         `json:"code"`
	Msg  string      `json:"msg"`
	T    int         `json:"t"`
	V    string      `json:"v"`
	Data interface{} `json:"data"`
}

type RespValue struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}

// Error writes err into w.
func Error(w http.ResponseWriter, err error) {
	lock.RLock()
	handler := errorHandler
	lock.RUnlock()

	if handler == nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	code, body := errorHandler(err)
	e, ok := body.(error)
	if ok {
		http.Error(w, e.Error(), code)
	} else {
		WriteJson(w, code, body)
	}
}

// Ok writes HTTP 200 OK into w.
func Ok(w http.ResponseWriter) {
	w.WriteHeader(http.StatusOK)
}

// OkJson writes v into w with 200 OK.
func OkJson(w http.ResponseWriter, v interface{}) {
	WriteJson(w, http.StatusOK, v)
}

// SetErrorHandler sets the error handler, which is called on calling Error.
func SetErrorHandler(handler func(error) (int, interface{})) {
	lock.Lock()
	defer lock.Unlock()
	errorHandler = handler
}

// WriteJson writes v as json string into w with code.
func WriteJson(w http.ResponseWriter, code int, v interface{}) {
	w.Header().Set(ContentType, ApplicationJson)
	w.WriteHeader(code)

	if bs, err := json.Marshal(v); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	} else if n, err := w.Write(bs); err != nil {
		// http.ErrHandlerTimeout has been handled by http.TimeoutHandler,
		// so it's ignored here.
		if err != http.ErrHandlerTimeout {
			logx.Errorf("write response failed, error: %s", err)
		}
	} else if n < len(bs) {
		logx.Errorf("actual bytes: %d, written bytes: %d", len(bs), n)
	}
}

//自定义返回参数模板
//
// Error writes err into w.
func RespJsonError(w http.ResponseWriter, err error) {
	lock.RLock()
	handler := errorHandler
	lock.RUnlock()
	t := int(time.Now().Unix())
	if handler == nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	code, body := errorHandler(err)
	e, ok := body.(error)
	if ok {
		http.Error(w, e.Error(), code)
	} else {
		WriteJson(w, http.StatusOK, RespJsonStruct{
			Code: code,
			Msg:  "",
			T:    t,
			V:    "1.0",
			Data: body,
		})
	}
}

func RespJson(w http.ResponseWriter, err error, v interface{}, code int, msg string, version string) {
	t := int(time.Now().Unix())
	if code == 0 {
		code = 200
		msg = "success"
	}
	if IsNil(v) {
		v = make(map[string]interface{}, 0)
	}
	if len(version) == 0 {
		version = "1.0"
	}

	WriteJson(w, http.StatusOK, RespJsonStruct{
		Code: code,
		Msg:  msg,
		T:    t,
		V:    version,
		Data: v,
	})
}

func IsNil(i interface{}) bool {
	vi := reflect.ValueOf(i)
	if vi.Kind() == reflect.Ptr {
		return vi.IsNil()
	}
	return false
}
