package fail2ban

import (
	"embed"
	"text/template"

	"github.com/tpodg/settled/internal/strutil"
)

//go:embed scripts/*.sh.tmpl
var fail2banScriptsFS embed.FS

var fail2banScriptTemplates = template.Must(template.New("fail2ban").Funcs(template.FuncMap{
	"shellEscape": strutil.ShellEscape,
}).Option("missingkey=error").ParseFS(fail2banScriptsFS, "scripts/*.sh.tmpl"))
