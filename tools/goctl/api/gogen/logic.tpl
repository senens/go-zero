package {{.pkgName}}

import (
	"net/http"

	{{.imports}}
)

type {{.logic}} struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
	Code int
    Msg string
    Version string
    Header  http.Header
}

func New{{.logic}}(ctx context.Context, svcCtx *svc.ServiceContext, header http.Header) *{{.logic}} {
	return &{{.logic}}{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
		Header: header,
	}
}

func (l *{{.logic}}) {{.function}}({{.request}}) {{.responseType}} {
	// todo: add your logic here and delete this line

	{{.returnString}}
}
