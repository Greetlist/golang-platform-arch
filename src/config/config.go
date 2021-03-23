package config

import (
    "gopkg.in/ini.v1"
    "path"
)

type ConfigLoader struct {
    // config type refer to ini config name, "tcp type" refer to tcp.ini etc.
    ConfigType string
    // ini pkg real config
    configFile *ini.File
}

func (config *ConfigLoader) Init() error {
    var err error
    config.configFile, err = config.loadConfig()
    return err
}

func (config *ConfigLoader) loadConfig() (*ini.File, error) {
    configFilename := config.ConfigType + ".ini"
    configPath := path.Join("conf", configFilename)
    return ini.Load(configPath)
}

func (config *ConfigLoader) GetInt(section, key string) int {
    return config.configFile.Section(section).Key(key).MustInt(0)
}

func (config *ConfigLoader) GetString(section, key string) string {
    return config.configFile.Section(section).Key(key).MustString("")
}
