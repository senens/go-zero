package {{.PkgName}}

import (
	"net/http"

	"github.com/zeromicro/go-zero/rest/httpx"
	{{.ImportPackages}}
)

func {{.HandlerName}}(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		{{if .HasRequest}}var req types.{{.RequestType}}
		{{end}}l := {{.LogicName}}.New{{.LogicType}}(r.Context(), svcCtx, r.Header)
		if err := httpx.Parse(r, &req); err != nil {
			httpx.RespJsonError(w, err,l.Secret)
			return
		}


		{{if .HasResp}}resp, {{end}}err := l.{{.Call}}({{if .HasRequest}}&req{{end}})
		if err != nil {
			httpx.RespJsonError(w, err,l.Secret)
		} else {
			{{if .HasResp}}httpx.RespJson(w, nil, resp, l.Code, l.Msg, l.Version,l.Secret){{end}}
		}
	}
}
