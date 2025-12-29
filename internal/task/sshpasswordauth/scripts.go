package sshpasswordauth

import (
	"embed"
	"text/template"

	"github.com/tpodg/settled/internal/strutil"
)

//go:embed scripts/*.sh.tmpl
var sshPasswordAuthScriptsFS embed.FS

var sshPasswordAuthScriptTemplates = template.Must(template.New("sshpasswordauth").Funcs(template.FuncMap{
	"shellEscape": strutil.ShellEscape,
}).Option("missingkey=error").ParseFS(sshPasswordAuthScriptsFS, "scripts/*.sh.tmpl"))
