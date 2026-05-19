package service

import "launchs/shared/templates"

// TemplateDefinition is re-exported from shared/templates for handler use.
type TemplateDefinition = templates.Template

func ListTemplates() []templates.Template {
	return templates.List()
}

func GetTemplateByID(id string) (templates.Template, bool) {
	return templates.GetByID(id)
}

func templateEnvVarsJSON(id string, userProvided map[string]string) string {
	return templates.ResolvedEnvVarsJSON(id, userProvided)
}

func templatePortsJSON(id string) string {
	return templates.DefaultPortsJSON(id)
}

func templateResourcesJSON(id string) string {
	return templates.ResourcesJSON(id)
}
