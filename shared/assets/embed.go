package assets

import "embed"

//go:embed templates/*.yaml
var TemplateFS embed.FS // テンプレート YAML ファイルを埋め込む
