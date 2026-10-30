package fix

import (
	"fmt"
	"strings"
	"tcpconnect/assets/fixspec"
)

// ResolveDictionary uses an imported extension when supplied, otherwise the
// built-in standard for the explicitly selected session version.
func ResolveDictionary(version string, data []byte) (*Dictionary, error) {
	if len(data) == 0 {
		if !SupportsVersion(version) {
			return nil, fmt.Errorf("请选择支持的 FIX 协议版本（4.0～4.4）")
		}
		name := strings.ReplaceAll(version, ".", "") + ".xml"
		if version == "FIX.4.4" {
			name = "standard/" + name
		}
		var err error
		data, err = fixspec.Dictionaries.ReadFile(name)
		if err != nil {
			return nil, err
		}
	}
	d, err := ParseDictionary(data)
	if err != nil {
		return nil, err
	}
	if version != "" && d.Version != version {
		return nil, fmt.Errorf("协议版本 %s 与自定义字典 %s 不匹配", version, d.Version)
	}
	return d, nil
}
