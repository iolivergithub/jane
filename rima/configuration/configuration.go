package configuration

type ConfigurationStruct struct {
	JaneURL   string
	Port      string
	DBFile    string
	ScriptDir string
	ListenOn  string
}

var ConfigData *ConfigurationStruct
