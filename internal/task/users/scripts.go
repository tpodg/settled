package users

import (
	"embed"
	"text/template"

	"github.com/tpodg/settled/internal/strutil"
)

//go:embed scripts/*.tmpl
var userScriptsFS embed.FS

var userScriptTemplates = template.Must(template.New("user").Funcs(template.FuncMap{
	"shellEscape": strutil.ShellEscape,
}).ParseFS(userScriptsFS, "scripts/*.tmpl"))
