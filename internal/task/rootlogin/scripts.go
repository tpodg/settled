package rootlogin

import (
	"embed"
	"text/template"

	"github.com/tpodg/settled/internal/strutil"
)

//go:embed scripts/*.sh.tmpl
var rootLoginScriptsFS embed.FS

var rootLoginScriptTemplates = template.Must(template.New("rootlogin").Funcs(template.FuncMap{
	"shellEscape": strutil.ShellEscape,
}).Option("missingkey=error").ParseFS(rootLoginScriptsFS, "scripts/*.sh.tmpl"))
