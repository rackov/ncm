package cfg

import (
	"os"

	"github.com/naoina/toml"
)

// Блок конфигурации
type NdServer struct {
	Name   string
	Port   int
	Active bool
}

type SqlServer struct {
	Driver    string
	Host      string
	Port      int
	User      string
	Password  string
	Dbname    string
	Lenbuf    int
	Ins_value string
}

type TomlConfig struct {
	Title       string
	Localport   int
	Index_head  int
	Sql_active  bool
	Sql_param   SqlServer
	Server      []NdServer
	Levellog    int
	PortControl int
}

func (t TomlConfig) Open_cfg(fname string) (TomlConfig, error) {
	var tom TomlConfig
	f, err := os.Open(fname)

	if err != nil {
		return tom, err
	}
	defer f.Close()

	if err := toml.NewDecoder(f).Decode(&tom); err != nil {
		return tom, err
	}
	return tom, err
}
