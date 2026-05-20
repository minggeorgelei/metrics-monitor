package models

func logName(pluginType, name, alias string) string {
	if alias == "" {
		return pluginType + "." + name
	}
	return pluginType + "." + name + "::" + alias
}
